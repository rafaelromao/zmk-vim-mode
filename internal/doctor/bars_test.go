package doctor

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHammerspoonChecks(t *testing.T) {
	if got := hammerspoonChecks(false, t.TempDir()); got[0].res != info {
		t.Errorf("no Hammerspoon: %+v", got)
	}
	dir := t.TempDir()
	if got := hammerspoonChecks(true, dir); got[0].res != warn {
		t.Errorf("no Spoon: %+v", got)
	}
	write(t, filepath.Join(dir, "Spoons", "ZmkVimMode.spoon", "init.lua"), "return {}\n")
	write(t, filepath.Join(dir, "init.lua"), "hs.alert.show(\"hello\")\n")
	if got := hammerspoonChecks(true, dir); got[0].res != warn {
		t.Errorf("Spoon never started: %+v", got)
	}
	write(t, filepath.Join(dir, "init.lua"), "hs.loadSpoon(\"ZmkVimMode\"):start()\n")
	if got := hammerspoonChecks(true, dir); got[0].res != ok {
		t.Errorf("all set: %+v", got)
	}
}

func TestOmarchyChecks(t *testing.T) {
	home := t.TempDir()
	if got := omarchyChecks(home); got[0].res != info {
		t.Errorf("no Omarchy: %+v", got)
	}
	shell := filepath.Join(home, ".config", "omarchy", "shell.json")
	write(t, shell, `{"bar":{"layout":{"right":[{"id":"omarchy.tray"}]}}}`)
	if got := omarchyChecks(home); got[0].res != warn {
		t.Errorf("no plugin: %+v", got)
	}
	write(t, filepath.Join(home, ".config", "omarchy", "plugins", "rafaelromao.zmk-vim-mode", "manifest.json"), "{}")
	if got := omarchyChecks(home); got[0].res != warn {
		t.Errorf("plugin not in the bar: %+v", got)
	}
	write(t, shell, `{"bar":{"layout":{"right":[{"id":"rafaelromao.zmk-vim-mode"},{"id":"omarchy.tray"}]}}}`)
	if got := omarchyChecks(home); got[0].res != ok {
		t.Errorf("all set: %+v", got)
	}
}
