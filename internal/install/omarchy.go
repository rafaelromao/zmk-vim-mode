package install

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	omarchybar "github.com/rafaelromao/zmk-vim-mode/bars/omarchy"
)

// OmarchyPluginID is the Quattro bar widget's plugin id. Omarchy wants
// third-party ids as <author>.<name>, lowercase, with the plugin's directory
// named after it; listing the id in the bar layout of shell.json is what
// enables it.
const OmarchyPluginID = "rafaelromao.zmk-vim-mode"

// OmarchyConfigDir is Omarchy's per-user configuration, where shell.json and
// the user's plugins live.
func OmarchyConfigDir(home string) string {
	return filepath.Join(home, ".config", "omarchy")
}

// OmarchyPluginDir is where the Quattro shell finds the widget.
func OmarchyPluginDir(home string) string {
	return filepath.Join(OmarchyConfigDir(home), "plugins", OmarchyPluginID)
}

// errNoRightSection means shell.json has no bar.layout.right array whose style
// an entry can be written in: missing, empty, or all on one line.
var errNoRightSection = errors.New("no multi-line bar.layout.right array to add the widget to")

// installOmarchyPlugin writes the plugin into dir with bin as its binary and
// reports whether anything changed. Every file lands as a regular file:
// Omarchy's plugin validator refuses symlinks inside a plugin, so a copy made
// of links would not load. A plugin directory that is itself a symlink is a
// development link into a checkout and is left alone (linked).
func installOmarchyPlugin(dir, bin string) (copied, linked bool, err error) {
	if isSymlink(dir) {
		return false, true, nil
	}
	src, err := fs.Sub(omarchybar.Files, OmarchyPluginID)
	if err != nil {
		return false, false, err
	}
	copied, err = writeAssets(src, dir, bin)
	return copied, false, err
}

// OmarchyWidgetEnabled reports whether a shell.json puts the widget in the bar.
func OmarchyWidgetEnabled(shellJSON []byte) (bool, error) {
	return shellHasWidget(shellJSON, OmarchyPluginID)
}

// shellHasWidget reports whether shell.json already enables the plugin id: as
// a layout entry, either {"id": ...} or a bare string, or in plugins.
func shellHasWidget(raw []byte, id string) (bool, error) {
	var cfg struct {
		Bar struct {
			Layout map[string][]json.RawMessage `json:"layout"`
		} `json:"bar"`
		Plugins []json.RawMessage `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false, err
	}
	is := func(m json.RawMessage) bool {
		var s string
		if json.Unmarshal(m, &s) == nil {
			return s == id
		}
		var e struct {
			ID string `json:"id"`
		}
		return json.Unmarshal(m, &e) == nil && e.ID == id
	}
	for _, section := range cfg.Bar.Layout {
		for _, m := range section {
			if is(m) {
				return true, nil
			}
		}
	}
	for _, m := range cfg.Plugins {
		if is(m) {
			return true, nil
		}
	}
	return false, nil
}

// enableOmarchyWidget puts the widget in the bar by listing its id first in
// bar.layout.right of shell.json, and reports whether it had to. An id that is
// already in the layout, anywhere, is where the user wants it.
//
// The file is edited as text, not re-encoded: encoding/json would reorder the
// keys and restyle a file people edit by hand. It is written in place rather
// than through a temporary file and a rename, so a shell.json that is a
// symlink stays one; the backup goes beside the link, not beside the file it
// points to.
func enableOmarchyWidget(path, id string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	present, err := shellHasWidget(raw, id)
	if err != nil {
		return false, fmt.Errorf("%s: %w", TrimHome(path), err)
	}
	if present {
		return false, nil
	}
	out, err := insertFirstRight(raw, id)
	if err != nil {
		return false, err
	}
	// Belt and braces: never write a file the shell would fail to read.
	if ok, err := shellHasWidget(out, id); err != nil || !ok {
		return false, fmt.Errorf("adding the widget to %s would break it; add {\"id\": %q} to bar.layout yourself", TrimHome(path), id)
	}
	if err := os.WriteFile(path+".bak-zmk-vim-mode", raw, 0o600); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, out, 0o644)
}

// insertFirstRight adds {"id": id} as the first entry of bar.layout.right, in
// the indentation the array's entries already use.
func insertFirstRight(raw []byte, id string) ([]byte, error) {
	at, err := rightArrayStart(raw)
	if err != nil {
		return nil, err
	}
	rest := raw[at:]
	next := bytes.TrimLeft(rest, " \t\r\n")
	ws := rest[:len(rest)-len(next)]
	nl := bytes.LastIndexByte(ws, '\n')
	if nl < 0 || len(next) == 0 || next[0] == ']' {
		return nil, errNoRightSection
	}
	ind := string(ws[nl+1:])
	quoted, err := json.Marshal(id)
	if err != nil {
		return nil, err
	}
	entry := string(ws) + "{\n" + ind + indentUnit(raw, at, ind) + `"id": ` + string(quoted) + "\n" + ind + "},"
	out := make([]byte, 0, len(raw)+len(entry))
	out = append(out, raw[:at]...)
	out = append(out, entry...)
	return append(out, rest...), nil
}

// rightArrayStart returns the offset just past the '[' that opens
// bar.layout.right, found by walking the tokens with a stack of the keys that
// lead to each nested value.
func rightArrayStart(raw []byte) (int, error) {
	type frame struct {
		object  bool   // strings alternate key, value
		wantKey bool   // the next string is a key
		key     string // the key whose value is being read
	}
	var stack []frame
	// valueDone marks the end of a value: in an object, a key comes next.
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].wantKey = true
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			return 0, errNoRightSection
		}
		n := len(stack)
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '[':
				if n == 3 && stack[0].key == "bar" && stack[1].key == "layout" && stack[2].key == "right" {
					return int(dec.InputOffset()), nil
				}
				stack = append(stack, frame{})
			case '{':
				stack = append(stack, frame{object: true, wantKey: true})
			default: // ']' or '}'
				stack = stack[:n-1]
				valueDone()
			}
		case string:
			if n > 0 && stack[n-1].object && stack[n-1].wantKey {
				stack[n-1].key, stack[n-1].wantKey = t, false
			} else {
				valueDone()
			}
		default:
			valueDone()
		}
	}
}

// indentUnit is one level of the file's indentation, measured from the line
// holding the array's '[' to the entries under it.
func indentUnit(raw []byte, at int, entryIndent string) string {
	line := raw[bytes.LastIndexByte(raw[:at], '\n')+1 : at]
	lead := string(line[:len(line)-len(bytes.TrimLeft(line, " \t"))])
	if strings.HasPrefix(entryIndent, lead) && len(entryIndent) > len(lead) {
		return entryIndent[len(lead):]
	}
	return "  "
}

// omarchyCommand runs one of Omarchy's own commands with a deadline, so a
// shell that is stuck cannot hang the install.
func omarchyCommand(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// InstallOmarchy installs the Quattro bar widget: the plugin, with this
// binary's path written into it, and its entry in the bar layout.
func InstallOmarchy(w io.Writer, exe string) error {
	if runtime.GOOS != "linux" {
		fmt.Fprintln(w, "omarchy    : Linux only; nothing to do")
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	shellJSON := filepath.Join(OmarchyConfigDir(home), "shell.json")
	_, statErr := os.Stat(shellJSON)
	if statErr != nil && !haveCommand("omarchy-shell") {
		fmt.Fprintln(w, "omarchy    : no Omarchy Quattro shell here; nothing to do")
		fmt.Fprintln(w, "             another bar can run `zmk-vim-mode status --bar`, e.g. a Waybar custom module")
		fmt.Fprintln(w, "             with \"return-type\": \"json\" and \"format\": \"\\ue6ae {}\"")
		return nil
	}
	dir := OmarchyPluginDir(home)
	_, statManifest := os.Stat(filepath.Join(dir, "manifest.json"))
	updated := statManifest == nil
	copied, linked, err := installOmarchyPlugin(dir, exe)
	if err != nil {
		return err
	}
	switch {
	case linked:
		fmt.Fprintf(w, "omarchy    : %s is a symlink (a development checkout); left alone\n", TrimHome(dir))
	case copied:
		fmt.Fprintf(w, "omarchy    : installed %s\n", TrimHome(dir))
	default:
		fmt.Fprintf(w, "omarchy    : %s is up to date\n", TrimHome(dir))
	}
	if copied {
		if haveCommand("omarchy") {
			if out, err := omarchyCommand("omarchy", "plugin", "validate", dir); err != nil {
				fmt.Fprintf(w, "omarchy    : omarchy plugin validate rejects it (%v):\n%s\n", err, strings.TrimSpace(string(out)))
			} else {
				fmt.Fprintln(w, "omarchy    : omarchy plugin validate accepts it")
			}
		}
		// The shell only discovers new plugin folders on a rescan.
		if haveCommand("omarchy-shell") {
			_, _ = omarchyCommand("omarchy-shell", "shell", "rescanPlugins")
		}
	}
	if statErr != nil {
		// With no shell.json of its own, the user runs Omarchy's default, and the
		// shell does not merge a partial file into it: writing one here would
		// throw the default layout away. Omarchy's own command copes.
		fmt.Fprintf(w, "omarchy    : no %s yet; put the widget in the bar with:\n", TrimHome(shellJSON))
		fmt.Fprintf(w, "               omarchy plugin enable %s\n", OmarchyPluginID)
		return nil
	}
	added, err := enableOmarchyWidget(shellJSON, OmarchyPluginID)
	switch {
	case errors.Is(err, errNoRightSection):
		fmt.Fprintf(w, "omarchy    : %s has %v;\n", TrimHome(shellJSON), err)
		fmt.Fprintf(w, "             add {\"id\": %q} to a section of bar.layout yourself\n", OmarchyPluginID)
	case err != nil:
		return err
	case added:
		fmt.Fprintf(w, "omarchy    : added it to the bar, first in bar.layout.right of %s%s\n", TrimHome(shellJSON), linkNote(shellJSON))
	default:
		fmt.Fprintf(w, "omarchy    : %s already has it in the bar\n", TrimHome(shellJSON))
	}
	if copied && updated {
		// The shell caches QML it has loaded; a changed widget may need this.
		fmt.Fprintln(w, "             if the bar still shows the old widget: omarchy restart shell")
	}
	return nil
}
