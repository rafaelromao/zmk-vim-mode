package install

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	vscodeext "github.com/rafaelromao/zmk-vim-mode/editors/vscode"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

// VSCode settings the daemon relies on. The title marker is how the daemon
// sees tool-window focus; accessibilitySupport "off" keeps Monaco out of
// screen-reader mode when the daemon turns the accessibility bus on.
var vscodeSettings = []struct {
	key   string
	value string // JSON literal
	// ensure, when set, adapts an existing value instead of replacing it.
	ensure func(current string) (string, bool)
}{
	{key: "window.title", value: quote(state.VSCodeWindowTitle), ensure: ensureTitleMarker},
	{key: "editor.accessibilitySupport", value: `"off"`},
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ensureTitleMarker keeps the user's title template and appends the marker.
func ensureTitleMarker(current string) (string, bool) {
	var cur string
	if err := json.Unmarshal([]byte(current), &cur); err != nil {
		return "", false
	}
	if strings.HasSuffix(strings.TrimSpace(cur), "[${focusedView}]") {
		return current, false
	}
	return quote(strings.TrimRight(cur, " ") + " [${focusedView}]"), true
}

// VSCodeUserDirs lists the User settings directories of VSCode flavours.
func VSCodeUserDirs(home string) []string {
	base := filepath.Join(home, ".config")
	if runtime.GOOS == "darwin" {
		base = filepath.Join(home, "Library", "Application Support")
	}
	return []string{
		filepath.Join(base, "Code", "User"),
		filepath.Join(base, "Code - OSS", "User"),
		filepath.Join(base, "VSCodium", "User"),
	}
}

// SetJSONCKey sets key to value (a JSON literal) in a settings.json text,
// preserving everything else -- comments and trailing commas included, which
// is why this is textual rather than a JSON round trip. An existing value is
// passed through ensure when given. It reports whether the text changed.
func SetJSONCKey(text, key, value string, ensure func(string) (string, bool)) (string, bool) {
	re := regexp.MustCompile(`("` + regexp.QuoteMeta(key) + `"\s*:\s*)("(?:[^"\\]|\\.)*"|true|false|null|-?[0-9.eE+-]+)`)
	if m := re.FindStringSubmatchIndex(text); m != nil {
		current := text[m[4]:m[5]]
		next := value
		if ensure != nil {
			n, changed := ensure(current)
			if !changed {
				return text, false
			}
			next = n
		} else if current == value {
			return text, false
		}
		return text[:m[4]] + next + text[m[5]:], true
	}
	open := strings.IndexByte(text, '{')
	if open < 0 {
		return "{\n  \"" + key + "\": " + value + "\n}\n", true
	}
	rest := strings.TrimLeft(text[open+1:], " \t\r\n")
	if strings.HasPrefix(rest, "}") { // empty object: no comma
		return text[:open+1] + "\n  \"" + key + "\": " + value + "\n" + text[open+1:], true
	}
	return text[:open+1] + "\n  \"" + key + "\": " + value + "," + text[open+1:], true
}

// applySettings edits one settings.json, keeping a backup of the original.
func applySettings(path string) (changed []string, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		raw = []byte("{\n}\n")
	}
	text := string(raw)
	for _, s := range vscodeSettings {
		var c bool
		text, c = SetJSONCKey(text, s.key, s.value, s.ensure)
		if c {
			changed = append(changed, s.key)
		}
	}
	if len(changed) == 0 {
		return nil, nil
	}
	if len(raw) > 0 {
		if err := os.WriteFile(path+".bak-zmk-vim-mode", raw, 0o600); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return changed, os.WriteFile(path, []byte(text), 0o600)
}

// ---- vsix --------------------------------------------------------------------

type packageJSON struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Publisher   string `json:"publisher"`
	Engines     struct {
		VSCode string `json:"vscode"`
	} `json:"engines"`
}

// CompanionVersion returns the embedded companion's version.
func CompanionVersion() (string, error) {
	p, err := embeddedPackage()
	return p.Version, err
}

func embeddedPackage() (packageJSON, error) {
	var p packageJSON
	b, err := vscodeext.Files.ReadFile("package.json")
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(b, &p)
	return p, err
}

type vsixManifest struct {
	XMLName  xml.Name `xml:"PackageManifest"`
	Version  string   `xml:"Version,attr"`
	Xmlns    string   `xml:"xmlns,attr"`
	XmlnsD   string   `xml:"xmlns:d,attr"`
	Metadata struct {
		Identity struct {
			Language  string `xml:"Language,attr"`
			ID        string `xml:"Id,attr"`
			Version   string `xml:"Version,attr"`
			Publisher string `xml:"Publisher,attr"`
		} `xml:"Identity"`
		DisplayName  string `xml:"DisplayName"`
		Description  string `xml:"Description"`
		Categories   string `xml:"Categories"`
		GalleryFlags string `xml:"GalleryFlags"`
		Properties   struct {
			Property []struct {
				ID    string `xml:"Id,attr"`
				Value string `xml:"Value,attr"`
			} `xml:"Property"`
		} `xml:"Properties"`
	} `xml:"Metadata"`
	Installation struct {
		InstallationTarget struct {
			ID string `xml:"Id,attr"`
		} `xml:"InstallationTarget"`
	} `xml:"Installation"`
	Dependencies string `xml:"Dependencies"`
	Assets       struct {
		Asset []struct {
			Type        string `xml:"Type,attr"`
			Path        string `xml:"Path,attr"`
			Addressable string `xml:"Addressable,attr"`
		} `xml:"Asset"`
	} `xml:"Assets"`
}

// WriteVSIX packages the embedded companion as a .vsix (a zip with the two
// manifests vsce would write) and returns its path.
func WriteVSIX(dir string) (string, error) {
	p, err := embeddedPackage()
	if err != nil {
		return "", err
	}
	var man vsixManifest
	man.Version = "2.0.0"
	man.Xmlns = "http://schemas.microsoft.com/developer/vsx-schema/2011"
	man.XmlnsD = "http://schemas.microsoft.com/developer/vsx-schema-design/2011"
	man.Metadata.Identity.Language = "en-US"
	man.Metadata.Identity.ID = p.Name
	man.Metadata.Identity.Version = p.Version
	man.Metadata.Identity.Publisher = p.Publisher
	man.Metadata.DisplayName = p.DisplayName
	man.Metadata.Description = p.Description
	man.Metadata.Categories = "Other"
	man.Metadata.GalleryFlags = "Public"
	for _, kv := range [][2]string{
		{"Microsoft.VisualStudio.Code.Engine", p.Engines.VSCode},
		{"Microsoft.VisualStudio.Code.ExtensionDependencies", ""},
		{"Microsoft.VisualStudio.Code.ExtensionPack", ""},
		{"Microsoft.VisualStudio.Code.ExtensionKind", "ui"},
		{"Microsoft.VisualStudio.Code.LocalizedLanguages", "English"},
	} {
		man.Metadata.Properties.Property = append(man.Metadata.Properties.Property, struct {
			ID    string `xml:"Id,attr"`
			Value string `xml:"Value,attr"`
		}{kv[0], kv[1]})
	}
	man.Installation.InstallationTarget.ID = "Microsoft.VisualStudio.Code"
	for _, a := range [][2]string{
		{"Microsoft.VisualStudio.Code.Manifest", "extension/package.json"},
		{"Microsoft.VisualStudio.Services.Content.Details", "extension/README.md"},
	} {
		man.Assets.Asset = append(man.Assets.Asset, struct {
			Type        string `xml:"Type,attr"`
			Path        string `xml:"Path,attr"`
			Addressable string `xml:"Addressable,attr"`
		}{a[0], a[1], "true"})
	}
	manifest, err := xml.MarshalIndent(man, "", "  ")
	if err != nil {
		return "", err
	}
	const contentTypes = `<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension=".json" ContentType="application/json"/>
  <Default Extension=".vsixmanifest" ContentType="text/xml"/>
  <Default Extension=".js" ContentType="application/javascript"/>
  <Default Extension=".md" ContentType="text/markdown"/>
</Types>
`
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	out := filepath.Join(dir, fmt.Sprintf("%s-%s.vsix", p.Name, p.Version))
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	write := func(name string, data []byte) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	if err := write("extension.vsixmanifest", append([]byte(xml.Header), manifest...)); err != nil {
		return "", err
	}
	if err := write("[Content_Types].xml", []byte(contentTypes)); err != nil {
		return "", err
	}
	err = fs.WalkDir(vscodeext.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := vscodeext.Files.ReadFile(path)
		if err != nil {
			return err
		}
		return write("extension/"+path, b)
	})
	if err != nil {
		return "", err
	}
	if err := zw.Close(); err != nil {
		return "", err
	}
	return out, f.Close()
}

// ---- driving `code` -----------------------------------------------------------

// codeBinaries returns the VSCode CLIs found in PATH.
func codeBinaries() []string {
	var out []string
	for _, b := range []string{"code", "code-oss", "codium", "code-insiders"} {
		if _, err := exec.LookPath(b); err == nil {
			out = append(out, b)
		}
	}
	return out
}

func listExtensions(bin string) map[string]bool {
	out, err := exec.Command(bin, "--list-extensions").Output()
	set := map[string]bool{}
	if err != nil {
		return set
	}
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			set[strings.ToLower(l)] = true
		}
	}
	return set
}

// RendererFlag is the Chromium switch without which Electron never builds the
// accessibility tree of its web content: the bus flags alone reach GTK and Qt,
// not VSCode's DOM. Arch's `code` wrapper appends the lines of
// ~/.config/code-flags.conf to the command line.
const RendererFlag = "--force-renderer-accessibility"

// codeFlagFiles maps a VSCode flavour's config dir to its flags file.
var codeFlagFiles = map[string]string{
	"Code":       "code-flags.conf",
	"Code - OSS": "code-flags.conf",
	"VSCodium":   "codium-flags.conf",
}

// ensureRendererFlag adds RendererFlag to the flags file of every installed
// flavour. It reports the files it changed.
func ensureRendererFlag(home string) ([]string, error) {
	base := filepath.Join(home, ".config")
	var changed []string
	seen := map[string]bool{}
	for dir, file := range codeFlagFiles {
		if _, err := os.Stat(filepath.Join(base, dir)); err != nil {
			continue
		}
		p := filepath.Join(base, file)
		if seen[p] {
			continue
		}
		seen[p] = true
		raw, err := os.ReadFile(p)
		if err != nil && !os.IsNotExist(err) {
			return changed, err
		}
		if strings.Contains(string(raw), RendererFlag) {
			continue
		}
		text := string(raw)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += RendererFlag + "\n"
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			return changed, err
		}
		changed = append(changed, p)
	}
	return changed, nil
}

// HasRendererFlag reports whether the flags file of some flavour carries it.
func HasRendererFlag(home string) bool {
	for _, file := range codeFlagFiles {
		raw, err := os.ReadFile(filepath.Join(home, ".config", file))
		if err == nil && strings.Contains(string(raw), RendererFlag) {
			return true
		}
	}
	return false
}

// InstallVSCode applies the settings, installs the companion (packaged on the
// fly) and vscode-neovim, and reports what still needs a human. With atspi it
// also adds the renderer accessibility flag VSCode needs to appear on the bus.
func InstallVSCode(w io.Writer, atspi bool) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if atspi {
		changed, err := ensureRendererFlag(home)
		if err != nil {
			return fmt.Errorf("code-flags.conf: %w", err)
		}
		if len(changed) == 0 {
			fmt.Fprintf(w, "flags      : %s already in the code-flags.conf files\n", RendererFlag)
		} else {
			fmt.Fprintf(w, "flags      : added %s to %s (Electron exposes its DOM to the accessibility bus only with it)\n", RendererFlag, strings.Join(changed, ", "))
		}
	}
	// 1. settings, in every flavour that has a User dir
	applied := false
	for _, d := range VSCodeUserDirs(home) {
		if _, err := os.Stat(d); err != nil {
			continue
		}
		applied = true
		p := filepath.Join(d, "settings.json")
		changed, err := applySettings(p)
		if err != nil {
			return fmt.Errorf("%s: %w", TrimHome(p), err)
		}
		if len(changed) == 0 {
			fmt.Fprintf(w, "settings   : %s already has %s\n", TrimHome(p), "window.title marker, editor.accessibilitySupport")
		} else {
			fmt.Fprintf(w, "settings   : %s — set %s (backup: settings.json.bak-zmk-vim-mode)\n", TrimHome(p), strings.Join(changed, ", "))
		}
	}
	if !applied {
		fmt.Fprintln(w, "settings   : no VSCode User directory found; nothing written")
	}

	// 2. extensions
	bins := codeBinaries()
	if len(bins) == 0 {
		fmt.Fprintln(w, "extensions : no `code` CLI in PATH; install the companion by hand (editors/vscode/README.md)")
		return nil
	}
	vsix, err := WriteVSIX(filepath.Join(os.TempDir(), "zmk-vim-mode"))
	if err != nil {
		return fmt.Errorf("package companion: %w", err)
	}
	for _, bin := range bins {
		exts := listExtensions(bin)
		out, err := exec.Command(bin, "--install-extension", vsix, "--force").CombinedOutput()
		if err != nil {
			fmt.Fprintf(w, "%-11s: companion install failed: %v\n%s", bin, err, out)
		} else {
			fmt.Fprintf(w, "%-11s: companion installed from %s\n", bin, TrimHome(vsix))
		}
		if exts["asvetliakov.vscode-neovim"] {
			fmt.Fprintf(w, "%-11s: vscode-neovim present\n", bin)
		} else {
			out, err := exec.Command(bin, "--install-extension", "asvetliakov.vscode-neovim").CombinedOutput()
			if err != nil {
				fmt.Fprintf(w, "%-11s: vscode-neovim install failed (network?): %v\n%s", bin, err, out)
			} else {
				fmt.Fprintf(w, "%-11s: vscode-neovim installed\n", bin)
			}
		}
		if exts["vscodevim.vim"] {
			fmt.Fprintf(w, "%-11s: VSCodeVim is installed and conflicts with vscode-neovim; remove it yourself:\n             %s --uninstall-extension vscodevim.vim\n", bin, bin)
		}
	}
	fmt.Fprintln(w, "\nrestart VSCode fully (not just reload) so the title template and the accessibility setting apply.")
	fmt.Fprintln(w, "then: zmk-vim-mode doctor")
	return nil
}
