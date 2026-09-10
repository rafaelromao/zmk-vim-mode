package install

import (
	"archive/zip"
	"os"
	"strings"
	"testing"
)

func TestSetJSONCKey(t *testing.T) {
	marker := quote("${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]")

	// Empty object: no comma.
	out, changed := SetJSONCKey("{}\n", "window.title", marker, ensureTitleMarker)
	if !changed || out != "{\n  \"window.title\": "+marker+"\n}\n" {
		t.Fatalf("empty object: %q", out)
	}
	// Existing members, a comment right after the brace, trailing comma: insert first.
	src := "{\n  // theme\n  \"workbench.colorTheme\": \"Tokyo Night\",\n}\n"
	out, changed = SetJSONCKey(src, "editor.accessibilitySupport", `"off"`, nil)
	if !changed || !strings.HasPrefix(out, "{\n  \"editor.accessibilitySupport\": \"off\",\n  // theme") {
		t.Fatalf("insert into non-empty: %q", out)
	}
	if !strings.Contains(out, "\"workbench.colorTheme\": \"Tokyo Night\",\n}") {
		t.Fatalf("rest must be untouched: %q", out)
	}
	// Existing title without marker: appended, template kept.
	src = `{ "window.title": "${activeEditorShort} — ${rootName}", "a": 1 }`
	out, changed = SetJSONCKey(src, "window.title", marker, ensureTitleMarker)
	want := `{ "window.title": "${activeEditorShort} — ${rootName} [${focusedView}]", "a": 1 }`
	if !changed || out != want {
		t.Fatalf("append marker:\n got %q\nwant %q", out, want)
	}
	// Already right: untouched.
	out2, changed := SetJSONCKey(out, "window.title", marker, ensureTitleMarker)
	if changed || out2 != out {
		t.Fatalf("idempotent title: %v %q", changed, out2)
	}
	// Existing plain value replaced; same value untouched.
	out, changed = SetJSONCKey(`{"editor.accessibilitySupport": "auto"}`, "editor.accessibilitySupport", `"off"`, nil)
	if !changed || out != `{"editor.accessibilitySupport": "off"}` {
		t.Fatalf("replace: %q", out)
	}
	if _, changed = SetJSONCKey(out, "editor.accessibilitySupport", `"off"`, nil); changed {
		t.Fatal("idempotent replace")
	}
	// Escaped quotes inside the existing value survive the match.
	out, _ = SetJSONCKey(`{"window.title": "a \"b\" c"}`, "window.title", marker, ensureTitleMarker)
	if !strings.Contains(out, `"a \"b\" c [${focusedView}]"`) {
		t.Fatalf("escaped quotes: %q", out)
	}
	// No object at all: create one.
	out, _ = SetJSONCKey("", "window.title", marker, nil)
	if out != "{\n  \"window.title\": "+marker+"\n}\n" {
		t.Fatalf("from empty: %q", out)
	}
}

func TestApplySettingsKeepsBackup(t *testing.T) {
	dir := t.TempDir()
	p := dir + "/settings.json"
	orig := "{\n  \"editor.fontSize\": 14,\n}\n"
	if err := os.WriteFile(p, []byte(orig), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := applySettings(p)
	if err != nil || len(changed) != 2 {
		t.Fatalf("apply: %v %v", changed, err)
	}
	bak, err := os.ReadFile(p + ".bak-zmk-vim-mode")
	if err != nil || string(bak) != orig {
		t.Fatalf("backup: %q %v", bak, err)
	}
	now, _ := os.ReadFile(p)
	if !strings.Contains(string(now), `"editor.fontSize": 14`) || !strings.Contains(string(now), `[${focusedView}]`) {
		t.Fatalf("result: %s", now)
	}
	if changed, _ := applySettings(p); len(changed) != 0 {
		t.Fatalf("second run must be a no-op: %v", changed)
	}
}

func TestWriteVSIX(t *testing.T) {
	path, err := WriteVSIX(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".vsix") || !strings.Contains(path, "zmk-vim-mode-") {
		t.Fatalf("name: %s", path)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got := map[string]bool{}
	for _, f := range r.File {
		got[f.Name] = true
	}
	for _, want := range []string{"extension.vsixmanifest", "[Content_Types].xml", "extension/package.json", "extension/extension.js", "extension/README.md"} {
		if !got[want] {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
	for _, f := range r.File {
		if f.Name != "extension.vsixmanifest" {
			continue
		}
		rc, _ := f.Open()
		b, _ := readAll(rc)
		rc.Close()
		s := string(b)
		v, _ := CompanionVersion()
		for _, want := range []string{`Id="zmk-vim-mode"`, `Publisher="rafaelromao"`, `Version="` + v + `"`, `Microsoft.VisualStudio.Code.Engine`, `extension/package.json`} {
			if !strings.Contains(s, want) {
				t.Fatalf("manifest lacks %s:\n%s", want, s)
			}
		}
	}
}

func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var out []byte
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		out = append(out, buf[:n]...)
		if err != nil {
			if err.Error() == "EOF" {
				return out, nil
			}
			return out, err
		}
	}
}
