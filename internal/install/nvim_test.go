package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindNvimSpec(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "nvim", "lua", "plugins")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if FindNvimSpec(home) != "" {
		t.Fatal("nothing should be found in an empty config")
	}
	other := filepath.Join(dir, "colors.lua")
	if err := os.WriteFile(other, []byte(`return { "folke/tokyonight.nvim" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if FindNvimSpec(home) != "" {
		t.Fatal("an unrelated spec must not match")
	}
	mine := filepath.Join(dir, "keyboard.lua")
	if err := os.WriteFile(mine, []byte(`return { "rafaelromao/zmk-vim-mode", opts = {} }`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := FindNvimSpec(home); got != mine {
		t.Fatalf("got %q want %q", got, mine)
	}
}

func TestNvimSpecContent(t *testing.T) {
	// The spec must opt into vscode-neovim, or LazyVim disables the plugin
	// there and VSCode falls back to legacy mode.
	if !strings.Contains(NvimSpec, "vscode = true") {
		t.Fatal("the printed spec must set vscode = true")
	}
	if !strings.Contains(NvimSpec, "lazy = false") {
		t.Fatal("the plugin must load eagerly")
	}
}
