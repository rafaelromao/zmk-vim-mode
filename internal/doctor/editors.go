package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/rafaelromao/zmk-vim-mode/internal/install"
	"github.com/rafaelromao/zmk-vim-mode/internal/state"
)

// editorChecks reports on the editor-side setups: VSCode (vscode-neovim, the
// window.title marker, the companion, the lazy spec) and Obsidian (the plugin
// in each vault). An editor that is not installed is info, not a failure.
func editorChecks(home string) []check {
	var cs []check
	cs = append(cs, vscodeChecks(home)...)
	cs = append(cs, obsidianChecks(home)...)
	return cs
}

// ---- VSCode ----------------------------------------------------------------

func vscodeUserDirs(home string) []string {
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

func vscodeExtensionDirs(home string) []string {
	return []string{
		filepath.Join(home, ".vscode", "extensions"),
		filepath.Join(home, ".vscode-oss", "extensions"),
	}
}

// windowTitleRe finds the window.title value in a settings.json, which may
// carry comments and trailing commas, so it is not parsed as JSON.
var windowTitleRe = regexp.MustCompile(`"window\.title"\s*:\s*"((?:[^"\\]|\\.)*)"`)

// windowTitleMarker returns the window.title template found in a settings.json
// text and whether it ends with the [${focusedView}] marker the daemon reads.
func windowTitleMarker(settings string) (title string, good bool) {
	m := windowTitleRe.FindStringSubmatch(settings)
	if m == nil {
		return "", false
	}
	title = m[1]
	return title, strings.HasSuffix(strings.TrimSpace(title), "[${focusedView}]")
}

// installedExtensions lists the extension folder names (lower case) found in
// the given extension directories.
func installedExtensions(dirs []string) []string {
	var names []string
	for _, d := range dirs {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			names = append(names, strings.ToLower(e.Name()))
		}
	}
	return names
}

func hasExtension(names []string, prefix string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}

// findLazySpec returns the lazy.nvim spec file mentioning zmk-vim-mode and its
// content, or "" when none is found under ~/.config/nvim/lua.
func findLazySpec(home string) (path, content string) {
	root := filepath.Join(home, ".config", "nvim", "lua")
	var found string
	var body string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" || d.IsDir() || !strings.HasSuffix(p, ".lua") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		if strings.Contains(string(b), "zmk-vim-mode") {
			found, body = p, string(b)
		}
		return nil
	})
	return found, body
}

func vscodeChecks(home string) []check {
	var settings string
	for _, d := range vscodeUserDirs(home) {
		p := filepath.Join(d, "settings.json")
		if _, err := os.Stat(p); err == nil {
			settings = p
			break
		}
	}
	if settings == "" {
		return []check{{"vscode", info, "not found (no User/settings.json)", ""}}
	}
	var cs []check
	b, err := os.ReadFile(settings)
	if err != nil {
		cs = append(cs, check{"vscode window.title", warn, "cannot read " + install.TrimHome(settings), ""})
	} else {
		title, good := windowTitleMarker(string(b))
		switch {
		case good:
			cs = append(cs, check{"vscode window.title", ok, "publishes [${focusedView}]", ""})
		case title == "":
			cs = append(cs, check{"vscode window.title", warn, "not set: tool-window focus is invisible",
				`add to ` + install.TrimHome(settings) + `: "window.title": "` + state.VSCodeWindowTitle + `"`})
		default:
			cs = append(cs, check{"vscode window.title", warn, "set, but does not end with [${focusedView}]",
				`make it end with " [${focusedView}]", e.g. "` + state.VSCodeWindowTitle + `"`})
		}
	}

	exts := installedExtensions(vscodeExtensionDirs(home))
	if hasExtension(exts, "asvetliakov.vscode-neovim-") {
		cs = append(cs, check{"vscode-neovim", ok, "installed", ""})
	} else {
		cs = append(cs, check{"vscode-neovim", warn, "not installed: VSCode stays legacy (keyboard infers modes)",
			"install asvetliakov.vscode-neovim; see editors/vscode/README.md"})
	}
	if hasExtension(exts, "vscodevim.vim-") {
		cs = append(cs, check{"vscodevim", warn, "installed: conflicts with vscode-neovim and reports no modes",
			"uninstall vscodevim.vim"})
	}
	if hasExtension(exts, "rafaelromao.zmk-vim-mode-") {
		cs = append(cs, check{"vscode companion", ok, "installed", ""})
	} else {
		cs = append(cs, check{"vscode companion", warn, "not installed: quick inputs and non-text editors keep the vim layers",
			"editors/vscode/README.md, step 3"})
	}

	spec, body := findLazySpec(home)
	switch {
	case spec == "":
		cs = append(cs, check{"nvim lazy spec", warn, "no file under ~/.config/nvim/lua mentions zmk-vim-mode",
			"zmk-vim-mode install --nvim prints the spec"})
	case !strings.Contains(body, "vscode = true"):
		cs = append(cs, check{"nvim lazy spec", warn, install.TrimHome(spec) + " lacks vscode = true",
			"LazyVim disables every other plugin inside vscode-neovim; add vscode = true to the spec"})
	default:
		cs = append(cs, check{"nvim lazy spec", ok, install.TrimHome(spec) + " (vscode = true)", ""})
	}
	return cs
}

// ---- Obsidian --------------------------------------------------------------

func obsidianConfigPath(home string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "obsidian", "obsidian.json")
	}
	return filepath.Join(home, ".config", "obsidian", "obsidian.json")
}

// obsidianVaults returns the vault paths listed in obsidian.json, sorted.
func obsidianVaults(cfg []byte) []string {
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

// communityPluginEnabled reports whether community-plugins.json lists id.
func communityPluginEnabled(list []byte, id string) bool {
	var ids []string
	if err := json.Unmarshal(list, &ids); err != nil {
		return false
	}
	for _, s := range ids {
		if s == id {
			return true
		}
	}
	return false
}

func obsidianChecks(home string) []check {
	cfg, err := os.ReadFile(obsidianConfigPath(home))
	if err != nil {
		return []check{{"obsidian", info, "not found (no obsidian.json)", ""}}
	}
	vaults := obsidianVaults(cfg)
	if len(vaults) == 0 {
		return []check{{"obsidian", info, "no vaults registered", ""}}
	}
	var cs []check
	for _, v := range vaults {
		name := "obsidian " + filepath.Base(v)
		pluginDir := filepath.Join(v, ".obsidian", "plugins", "zmk-vim-mode")
		_, installed := os.Stat(filepath.Join(pluginDir, "main.js"))
		list, _ := os.ReadFile(filepath.Join(v, ".obsidian", "community-plugins.json"))
		switch {
		case installed != nil:
			cs = append(cs, check{name, warn, "plugin not installed: Obsidian stays legacy",
				"copy editors/obsidian/{manifest.json,main.js} to " + install.TrimHome(pluginDir)})
		case !communityPluginEnabled(list, "zmk-vim-mode"):
			cs = append(cs, check{name, warn, "plugin installed but not enabled",
				"Settings → Community plugins → enable ZMK Vim Mode"})
		default:
			cs = append(cs, check{name, ok, "plugin installed and enabled", ""})
		}
	}
	return cs
}
