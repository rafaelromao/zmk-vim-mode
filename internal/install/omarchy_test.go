package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallOmarchyPlugin(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugins", OmarchyPluginID)
	copied, linked, err := installOmarchyPlugin(dir, "/home/me/.local/bin/zmk-vim-mode")
	if err != nil || !copied || linked {
		t.Fatalf("first install: copied=%v linked=%v err=%v", copied, linked, err)
	}
	var m struct {
		SchemaVersion int      `json:"schemaVersion"`
		ID            string   `json:"id"`
		Kinds         []string `json:"kinds"`
		EntryPoints   struct {
			BarWidget string `json:"barWidget"`
		} `json:"entryPoints"`
		BarWidget struct {
			DisplayName string `json:"displayName"`
		} `json:"barWidget"`
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest.json: %v", err)
	}
	if m.SchemaVersion != 1 || m.ID != OmarchyPluginID || filepath.Base(dir) != m.ID {
		t.Errorf("manifest: schemaVersion=%d id=%q in %s", m.SchemaVersion, m.ID, filepath.Base(dir))
	}
	if len(m.Kinds) != 1 || m.Kinds[0] != "bar-widget" || m.BarWidget.DisplayName == "" {
		t.Errorf("manifest does not declare a bar widget: %+v", m)
	}
	entry := filepath.Join(dir, m.EntryPoints.BarWidget)
	if st, err := os.Lstat(entry); err != nil || !st.Mode().IsRegular() {
		t.Fatalf("entry point %s: %v", m.EntryPoints.BarWidget, err)
	}
	qml, _ := os.ReadFile(entry)
	if !strings.Contains(string(qml), `moduleName: "`+OmarchyPluginID+`"`) {
		t.Error("the widget's moduleName is not its plugin id")
	}
	if !strings.Contains(string(qml), `"/home/me/.local/bin/zmk-vim-mode"`) {
		t.Error("the binary's path was not written into the widget")
	}
	copied, _, err = installOmarchyPlugin(dir, "/home/me/.local/bin/zmk-vim-mode")
	if err != nil || copied {
		t.Fatalf("second install must be a no-op: copied=%v err=%v", copied, err)
	}
}

// Omarchy's validator refuses symlinks inside a plugin, so a linked file is
// replaced by a real one, and the file it pointed to is left alone.
func TestInstallOmarchyPluginReplacesSymlinks(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "plugins", OmarchyPluginID)
	target := filepath.Join(root, "elsewhere", "manifest.json")
	mustWrite(t, target, "{}")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := installOmarchyPlugin(dir, "/bin/zmk-vim-mode"); err != nil {
		t.Fatal(err)
	}
	if isSymlink(filepath.Join(dir, "manifest.json")) {
		t.Error("manifest.json is still a symlink")
	}
	if b, _ := os.ReadFile(target); string(b) != "{}" {
		t.Errorf("wrote through the symlink: %q", b)
	}
}

func TestInstallOmarchyPluginLeavesDevLink(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(root, "checkout")
	mustWrite(t, filepath.Join(checkout, "VimMode.qml"), "// working copy\n")
	dir := filepath.Join(root, "plugins", OmarchyPluginID)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(checkout, dir); err != nil {
		t.Fatal(err)
	}
	copied, linked, err := installOmarchyPlugin(dir, "/bin/zmk-vim-mode")
	if err != nil || copied || !linked {
		t.Fatalf("copied=%v linked=%v err=%v", copied, linked, err)
	}
	if b, _ := os.ReadFile(filepath.Join(checkout, "VimMode.qml")); string(b) != "// working copy\n" {
		t.Errorf("the checkout was written to: %q", b)
	}
}

// The widget goes first in bar.layout.right, written the way the file writes
// its entries, and nothing else in the file moves.
func TestEnableOmarchyWidget(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "shell.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "shell.json")
	mustWrite(t, path, string(fixture))
	added, err := enableOmarchyWidget(path, OmarchyPluginID)
	if err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	got, _ := os.ReadFile(path)
	want := strings.Replace(string(fixture), "      \"right\": [\n",
		"      \"right\": [\n        {\n          \"id\": \"rafaelromao.zmk-vim-mode\"\n        },\n", 1)
	if string(got) != want {
		t.Errorf("shell.json:\n%s\nwant:\n%s", got, want)
	}
	if backup, _ := os.ReadFile(path + ".bak-zmk-vim-mode"); !bytes.Equal(backup, fixture) {
		t.Error("the backup is not the original file")
	}
	added, err = enableOmarchyWidget(path, OmarchyPluginID)
	if err != nil || added {
		t.Fatalf("second run must be a no-op: added=%v err=%v", added, err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(again, got) {
		t.Error("the second run changed the file")
	}
}

// A shell.json that is a symlink stays one: the edit lands in the file it
// points to, and the backup goes beside the link.
func TestEnableOmarchyWidgetWritesThroughSymlink(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "shell.json"))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere", "shell.json")
	mustWrite(t, target, string(fixture))
	link := filepath.Join(root, "omarchy", "shell.json")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if added, err := enableOmarchyWidget(link, OmarchyPluginID); err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	if !isSymlink(link) {
		t.Fatal("the symlink was replaced by a file")
	}
	b, _ := os.ReadFile(target)
	if ok, err := OmarchyWidgetEnabled(b); err != nil || !ok {
		t.Errorf("the linked shell.json does not have the widget (err %v)", err)
	}
	if _, err := os.Stat(link + ".bak-zmk-vim-mode"); err != nil {
		t.Error("no backup beside the link")
	}
	if _, err := os.Stat(target + ".bak-zmk-vim-mode"); err == nil {
		t.Error("the backup landed beside the file behind the link")
	}
}

func TestShellHasWidget(t *testing.T) {
	id := OmarchyPluginID
	for _, c := range []struct {
		name string
		json string
		want bool
	}{
		{"object entry", `{"bar":{"layout":{"center":[{"id":"x"},{"id":"` + id + `"}]}}}`, true},
		{"bare string entry", `{"bar":{"layout":{"left":["` + id + `"]}}}`, true},
		{"in plugins", `{"bar":{"layout":{}},"plugins":["` + id + `"]}`, true},
		{"absent", `{"bar":{"layout":{"right":[{"id":"omarchy.tray"}]}},"plugins":[]}`, false},
		{"no bar", `{"version":1}`, false},
	} {
		got, err := shellHasWidget([]byte(c.json), id)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v (err %v), want %v", c.name, got, err, c.want)
		}
	}
	if _, err := shellHasWidget([]byte(`{"bar":`), id); err == nil {
		t.Error("broken JSON must be an error, not \"absent\"")
	}
}

// Arrays this code cannot match the style of are refused, and left alone.
func TestEnableOmarchyWidgetRefuses(t *testing.T) {
	for name, content := range map[string]string{
		"empty right":    "{\n  \"bar\": {\n    \"layout\": {\n      \"right\": []\n    }\n  }\n}\n",
		"one-line right": "{\n  \"bar\": {\n    \"layout\": {\n      \"right\": [{\"id\": \"omarchy.tray\"}]\n    }\n  }\n}\n",
		"no right":       "{\n  \"bar\": {\n    \"layout\": {\n      \"left\": [\n        \"omarchy.tray\"\n      ]\n    }\n  }\n}\n",
	} {
		path := filepath.Join(t.TempDir(), "shell.json")
		mustWrite(t, path, content)
		added, err := enableOmarchyWidget(path, OmarchyPluginID)
		if added || !errors.Is(err, errNoRightSection) {
			t.Errorf("%s: added=%v err=%v", name, added, err)
		}
		if b, _ := os.ReadFile(path); string(b) != content {
			t.Errorf("%s: the file changed", name)
		}
		if _, err := os.Stat(path + ".bak-zmk-vim-mode"); err == nil {
			t.Errorf("%s: a backup was written for an edit that did not happen", name)
		}
	}
}

// Only bar.layout.right counts: a "right" key anywhere else is not it.
func TestRightArrayStartIgnoresOtherRights(t *testing.T) {
	raw := []byte("{\n  \"right\": [\n    1\n  ],\n  \"bar\": {\n    \"right\": [\n      2\n    ],\n" +
		"    \"layout\": {\n      \"left\": [\n        {\n          \"id\": \"x\",\n          \"right\": [\n            3\n          ]\n        }\n      ],\n" +
		"      \"right\": [\n        {\n          \"id\": \"omarchy.tray\"\n        }\n      ]\n    }\n  }\n}\n")
	at, err := rightArrayStart(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(raw[at:], []byte("\n        {\n          \"id\": \"omarchy.tray\"")) {
		t.Errorf("found the wrong array: %q", raw[at:min(len(raw), at+40)])
	}
}
