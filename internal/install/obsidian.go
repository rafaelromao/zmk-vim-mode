package install

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	obsidianplugin "github.com/rafaelromao/zmk-vim-mode/editors/obsidian"
)

const obsidianPluginID = "zmk-vim-mode"

// ObsidianConfigPath is where Obsidian lists the vaults it knows.
func ObsidianConfigPath(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json")
	}
	return filepath.Join(home, ".config", "obsidian", "obsidian.json")
}

// ObsidianVaults returns the vault paths listed in an obsidian.json, sorted.
func ObsidianVaults(cfg []byte) []string {
	var parsed struct {
		Vaults map[string]struct {
			Path string `json:"path"`
		} `json:"vaults"`
	}
	if err := json.Unmarshal(cfg, &parsed); err != nil {
		return nil
	}
	var out []string
	for _, v := range parsed.Vaults {
		if v.Path != "" {
			out = append(out, v.Path)
		}
	}
	sort.Strings(out)
	return out
}

// installObsidianPlugin copies the plugin into one vault and lists it among
// the enabled community plugins. It reports what changed.
func installObsidianPlugin(vault string) (copied, enabled bool, err error) {
	dir := filepath.Join(vault, ".obsidian", "plugins", obsidianPluginID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return false, false, err
	}
	for _, name := range []string{"manifest.json", "main.js"} {
		b, err := obsidianplugin.Files.ReadFile(name)
		if err != nil {
			return false, false, err
		}
		p := filepath.Join(dir, name)
		if old, err := os.ReadFile(p); err == nil && string(old) == string(b) {
			continue
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return false, false, err
		}
		copied = true
	}
	enabled, err = enableCommunityPlugin(filepath.Join(vault, ".obsidian", "community-plugins.json"), obsidianPluginID)
	return copied, enabled, err
}

// enableCommunityPlugin adds id to community-plugins.json (a JSON array of
// ids) unless present. Obsidian rereads the file on restart.
func enableCommunityPlugin(path, id string) (bool, error) {
	var ids []string
	raw, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(raw, &ids); err != nil {
			return false, fmt.Errorf("%s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	for _, s := range ids {
		if s == id {
			return false, nil
		}
	}
	if len(raw) > 0 {
		if err := os.WriteFile(path+".bak-zmk-vim-mode", raw, 0o600); err != nil {
			return false, err
		}
	}
	ids = append(ids, id)
	b, err := json.MarshalIndent(ids, "", "  ")
	if err != nil {
		return false, err
	}
	return true, os.WriteFile(path, append(b, '\n'), 0o644)
}

// InstallObsidian installs and enables the plugin in every vault Obsidian
// knows about.
func InstallObsidian(w io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfg, err := os.ReadFile(ObsidianConfigPath(home))
	if err != nil {
		fmt.Fprintln(w, "obsidian   : not found (no obsidian.json); copy editors/obsidian/{manifest.json,main.js} into <vault>/.obsidian/plugins/zmk-vim-mode/ by hand")
		return nil
	}
	vaults := ObsidianVaults(cfg)
	if len(vaults) == 0 {
		fmt.Fprintln(w, "obsidian   : no vaults registered")
		return nil
	}
	for _, v := range vaults {
		copied, enabled, err := installObsidianPlugin(v)
		if err != nil {
			return fmt.Errorf("%s: %w", TrimHome(v), err)
		}
		state := "already installed and enabled"
		switch {
		case copied && enabled:
			state = "installed and enabled"
		case copied:
			state = "updated"
		case enabled:
			state = "enabled"
		}
		fmt.Fprintf(w, "obsidian   : %s — %s\n", TrimHome(v), state)
	}
	fmt.Fprintln(w, "restart Obsidian (and keep Settings → Editor → Vim key bindings on); the plugin's *Status* command shows what it reports.")
	return nil
}
