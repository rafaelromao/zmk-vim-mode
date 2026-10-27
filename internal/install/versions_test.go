package install

import (
	"io/fs"
	"os"
	"regexp"
	"strings"
	"testing"

	hammerspoonbar "github.com/rafaelromao/zmk-vim-mode/bars/hammerspoon"
	omarchybar "github.com/rafaelromao/zmk-vim-mode/bars/omarchy"
	intellijplugin "github.com/rafaelromao/zmk-vim-mode/editors/intellij"
	obsidianplugin "github.com/rafaelromao/zmk-vim-mode/editors/obsidian"
	vscodeext "github.com/rafaelromao/zmk-vim-mode/editors/vscode"
)

// Every module ships under the release's version, so a release bumps them
// all at once. The daemon takes its version from the git tag instead.
func TestModuleVersionsMatch(t *testing.T) {
	jsonVersion := regexp.MustCompile(`"version":\s*"([^"]+)"`)
	jsPlugin := regexp.MustCompile(`const PLUGIN = '([^']+)'`)
	sources := []struct {
		name string
		fsys fs.FS // nil reads name from disk, relative to this package
		re   *regexp.Regexp
	}{
		{"lua/zmk-vim-mode/init.lua", nil, regexp.MustCompile(`M\.version = "([^"]+)"`)},
		{"editors/vscode/package.json", vscodeext.Files, jsonVersion},
		{"editors/vscode/extension.js", vscodeext.Files, jsPlugin},
		{"editors/obsidian/manifest.json", obsidianplugin.Files, jsonVersion},
		{"editors/obsidian/main.js", obsidianplugin.Files, jsPlugin},
		{"editors/intellij/build.gradle.kts", intellijplugin.Files, regexp.MustCompile(`(?m)^version = "([^"]+)"`)},
		{"editors/intellij/src/main/kotlin/dev/rafaelromao/zmkvimmode/Startup.kt", intellijplugin.Files, regexp.MustCompile(`PLUGIN_VERSION = "([^"]+)"`)},
		{"bars/omarchy/rafaelromao.zmk-vim-mode/manifest.json", omarchybar.Files, jsonVersion},
		{"bars/hammerspoon/ZmkVimMode.spoon/init.lua", hammerspoonbar.Files, regexp.MustCompile(`obj\.version = "([^"]+)"`)},
	}
	var want, wantFrom string
	for _, src := range sources {
		var b []byte
		var err error
		if src.fsys == nil {
			b, err = os.ReadFile("../../" + src.name)
		} else {
			b, err = fs.ReadFile(src.fsys, embeddedPath(src.name))
		}
		if err != nil {
			t.Fatalf("%s: %v", src.name, err)
		}
		m := src.re.FindSubmatch(b)
		if m == nil {
			t.Fatalf("%s: no version found", src.name)
		}
		got := string(m[1])
		if want == "" {
			want, wantFrom = got, src.name
			continue
		}
		if got != want {
			t.Errorf("%s is %s, but %s is %s", src.name, got, wantFrom, want)
		}
	}
}

// embeddedPath drops the module's directory (two segments, e.g.
// editors/vscode), which is the root of its embedded FS.
func embeddedPath(name string) string {
	return strings.SplitN(name, "/", 3)[2]
}
