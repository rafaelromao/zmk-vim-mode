package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallObsidianPlugin(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(vault, ".obsidian", "community-plugins.json")
	if err := os.WriteFile(list, []byte(`["obsidian-vimrc-support"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	copied, enabled, err := installObsidianPlugin(vault, false)
	if err != nil || !copied || !enabled {
		t.Fatalf("first install: copied=%v enabled=%v err=%v", copied, enabled, err)
	}
	for _, f := range []string{"manifest.json", "main.js"} {
		if _, err := os.Stat(filepath.Join(vault, ".obsidian", "plugins", "zmk-vim-mode", f)); err != nil {
			t.Fatalf("%s not copied: %v", f, err)
		}
	}
	b, _ := os.ReadFile(list)
	if !strings.Contains(string(b), `"obsidian-vimrc-support"`) || !strings.Contains(string(b), `"zmk-vim-mode"`) {
		t.Fatalf("list: %s", b)
	}
	if _, err := os.Stat(list + ".bak-zmk-vim-mode"); err != nil {
		t.Fatal("backup missing")
	}
	copied, enabled, err = installObsidianPlugin(vault, false)
	if err != nil || copied || enabled {
		t.Fatalf("second install must be a no-op: copied=%v enabled=%v err=%v", copied, enabled, err)
	}
}

// With Obsidian running the plugin list must be left alone: Obsidian rewrites
// it from memory when it quits, so an edit here is silently lost -- which is
// exactly how the plugin ended up installed but never enabled.
func TestInstallObsidianPluginLeavesListAloneWhileRunning(t *testing.T) {
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(vault, ".obsidian", "community-plugins.json")
	if err := os.WriteFile(list, []byte(`["dataview"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	copied, enabled, err := installObsidianPlugin(vault, true)
	if err != nil || !copied {
		t.Fatalf("files must still be copied: copied=%v err=%v", copied, err)
	}
	if enabled {
		t.Fatal("the list must not be touched while Obsidian runs")
	}
	b, _ := os.ReadFile(list)
	if string(b) != `["dataview"]` {
		t.Fatalf("list changed: %s", b)
	}
}

func TestEnableCommunityPluginFromNothing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "community-plugins.json")
	enabled, err := enableCommunityPlugin(p, "zmk-vim-mode")
	if err != nil || !enabled {
		t.Fatalf("%v %v", enabled, err)
	}
	b, _ := os.ReadFile(p)
	if strings.TrimSpace(string(b)) != "[\n  \"zmk-vim-mode\"\n]" {
		t.Fatalf("%q", b)
	}
}

func TestObsidianVaults(t *testing.T) {
	got := ObsidianVaults([]byte(`{"vaults":{"a":{"path":"/v/notes"},"b":{"path":"/v/work","open":true}}}`))
	if len(got) != 2 || got[0] != "/v/notes" || got[1] != "/v/work" {
		t.Fatalf("%v", got)
	}
}
