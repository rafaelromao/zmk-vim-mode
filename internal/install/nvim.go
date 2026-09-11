package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// NvimSpecPath is where the plugin spec is written when none exists.
func NvimSpecPath(home string) string {
	return filepath.Join(home, ".config", "nvim", "lua", "plugins", "zmk-vim-mode.lua")
}

// FindNvimSpec returns the first file under ~/.config/nvim/lua that mentions
// zmk-vim-mode, or "" when there is none.
func FindNvimSpec(home string) string {
	root := filepath.Join(home, ".config", "nvim", "lua")
	var found string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() || !strings.HasSuffix(p, ".lua") {
			return nil
		}
		if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), "zmk-vim-mode") {
			found = p
		}
		return nil
	})
	return found
}

// InstallNvim writes the lazy.nvim spec when the config has none. An existing
// spec is left alone whatever it contains: it is the user's, and a second one
// would make lazy.nvim load the plugin twice.
func InstallNvim(w io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if p := FindNvimSpec(home); p != "" {
		fmt.Fprintf(w, "nvim       : %s already specs zmk-vim-mode\n", TrimHome(p))
		return nil
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "nvim")); err != nil {
		fmt.Fprintln(w, "nvim       : no ~/.config/nvim here; add this spec wherever your config lives:")
		fmt.Fprint(w, NvimSpec)
		return nil
	}
	p := NvimSpecPath(home)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(NvimSpec), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(w, "nvim       : wrote %s — restart Neovim\n", TrimHome(p))
	return nil
}
