package install

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hammerspoonbar "github.com/rafaelromao/zmk-vim-mode/bars/hammerspoon"
	omarchybar "github.com/rafaelromao/zmk-vim-mode/bars/omarchy"
)

func TestInstallSpoon(t *testing.T) {
	spoons := filepath.Join(t.TempDir(), "Spoons")
	copied, linked, err := installSpoon(spoons, "/opt/zmk/bin/zmk-vim-mode")
	if err != nil || !copied || linked {
		t.Fatalf("first install: copied=%v linked=%v err=%v", copied, linked, err)
	}
	b, err := os.ReadFile(filepath.Join(spoons, "ZmkVimMode.spoon", "init.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `obj.binary = "/opt/zmk/bin/zmk-vim-mode"`) {
		t.Error("the binary's path was not written into the Spoon")
	}
	if strings.Contains(string(b), binaryPlaceholder) {
		t.Error("the placeholder survived the install")
	}
	copied, _, err = installSpoon(spoons, "/opt/zmk/bin/zmk-vim-mode")
	if err != nil || copied {
		t.Fatalf("second install must be a no-op: copied=%v err=%v", copied, err)
	}
	copied, _, err = installSpoon(spoons, "/usr/local/bin/zmk-vim-mode")
	if err != nil || !copied {
		t.Fatalf("another binary must rewrite the Spoon: copied=%v err=%v", copied, err)
	}
}

// A Spoon directory that is a symlink points into a checkout; writing the
// binary's path through it would edit the repository.
func TestInstallSpoonLeavesDevLink(t *testing.T) {
	root := t.TempDir()
	checkout := filepath.Join(root, "checkout", "ZmkVimMode.spoon")
	mustWrite(t, filepath.Join(checkout, "init.lua"), "-- working copy\n")
	spoons := filepath.Join(root, "Spoons")
	if err := os.MkdirAll(spoons, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(checkout, filepath.Join(spoons, "ZmkVimMode.spoon")); err != nil {
		t.Fatal(err)
	}
	copied, linked, err := installSpoon(spoons, "/opt/zmk/bin/zmk-vim-mode")
	if err != nil || copied || !linked {
		t.Fatalf("copied=%v linked=%v err=%v", copied, linked, err)
	}
	if b, _ := os.ReadFile(filepath.Join(checkout, "init.lua")); string(b) != "-- working copy\n" {
		t.Errorf("the checkout was written to: %q", b)
	}
}

// Each asset must carry the placeholder exactly once, on the line the
// installer means to fill: a second copy (a comparison, say) would be
// rewritten too.
func TestBarAssetsCarryOnePlaceholder(t *testing.T) {
	for _, a := range []struct {
		fsys fs.FS
		path string
	}{
		{hammerspoonbar.Files, "ZmkVimMode.spoon/init.lua"},
		{omarchybar.Files, OmarchyPluginID + "/VimMode.qml"},
	} {
		b, err := fs.ReadFile(a.fsys, a.path)
		if err != nil {
			t.Fatal(err)
		}
		if n := strings.Count(string(b), binaryPlaceholder); n != 1 {
			t.Errorf("%s has the placeholder %d times, want 1", a.path, n)
		}
	}
}

func TestWithBinaryEscapes(t *testing.T) {
	got := string(withBinary([]byte(`bin = "__ZMK_VIM_MODE_BIN__"`), `/o d/"q"\z`))
	if want := `bin = "/o d/\"q\"\\z"`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestEnsureSpoonLoaderCreates(t *testing.T) {
	initPath := filepath.Join(t.TempDir(), ".hammerspoon", "init.lua")
	added, err := ensureSpoonLoader(initPath)
	if err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	b, _ := os.ReadFile(initPath)
	if string(b) != strings.TrimPrefix(spoonLoader, "\n") {
		t.Errorf("a new init.lua holds just the block, got %q", b)
	}
	added, err = ensureSpoonLoader(initPath)
	if err != nil || added {
		t.Fatalf("second run must be a no-op: added=%v err=%v", added, err)
	}
}

func TestEnsureSpoonLoaderAppends(t *testing.T) {
	initPath := filepath.Join(t.TempDir(), "init.lua")
	mustWrite(t, initPath, `hs.alert.show("hello")`)
	added, err := ensureSpoonLoader(initPath)
	if err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	b, _ := os.ReadFile(initPath)
	if want := "hs.alert.show(\"hello\")\n" + spoonLoader; string(b) != want {
		t.Errorf("got %q, want %q", b, want)
	}
}

// Lines that already do something with the Spoon are the user's.
func TestEnsureSpoonLoaderLeavesUserLines(t *testing.T) {
	initPath := filepath.Join(t.TempDir(), "init.lua")
	user := "local vim = hs.loadSpoon(\"ZmkVimMode\")\nvim.interval = 2\nvim:start()\n"
	mustWrite(t, initPath, user)
	added, err := ensureSpoonLoader(initPath)
	if err != nil || added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	if b, _ := os.ReadFile(initPath); string(b) != user {
		t.Errorf("init.lua changed: %q", b)
	}
}

// An init.lua that is a symlink stays one: the block lands in the file it
// points to.
func TestEnsureSpoonLoaderWritesThroughSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "elsewhere", "init.lua")
	mustWrite(t, target, "hs.alert.show(\"hello\")\n")
	link := filepath.Join(root, ".hammerspoon", "init.lua")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	added, err := ensureSpoonLoader(link)
	if err != nil || !added {
		t.Fatalf("added=%v err=%v", added, err)
	}
	if !isSymlink(link) {
		t.Fatal("the symlink was replaced by a file")
	}
	if b, _ := os.ReadFile(target); string(b) != "hs.alert.show(\"hello\")\n"+spoonLoader {
		t.Errorf("the linked init.lua: %q", b)
	}
}

// The block must recognise itself, or every install would append another.
func TestSpoonLoaderIsRecognised(t *testing.T) {
	if !SpoonLoaded([]byte(spoonLoader)) {
		t.Fatal("SpoonLoaded does not see the block this package appends")
	}
}

func TestHammerspoonConfigDir(t *testing.T) {
	for in, want := range map[string]string{
		"":                               "/Users/me/.hammerspoon",
		"~/.config/hammerspoon/init.lua": "/Users/me/.config/hammerspoon",
		"/opt/hs/init.lua":               "/opt/hs",
	} {
		if got := HammerspoonConfigDir("/Users/me", in); got != want {
			t.Errorf("MJConfigFile %q: got %s, want %s", in, got, want)
		}
	}
}
