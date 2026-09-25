package doctor

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/rafaelromao/zmk-vim-mode/internal/install"
)

// barChecks reports on this platform's status bar indicator: the Hammerspoon
// Spoon on macOS, the Quattro bar widget on Omarchy. A host that is not
// installed is info, not a failure.
func barChecks(home string) []check {
	switch runtime.GOOS {
	case "darwin":
		return hammerspoonChecks(install.HammerspoonInstalled(home),
			install.HammerspoonConfigDir(home, install.HammerspoonConfigFile()))
	case "linux":
		return omarchyChecks(home)
	}
	return nil
}

// hammerspoonChecks looks for the Spoon in dir, Hammerspoon's config
// directory, and for init.lua starting it.
func hammerspoonChecks(installed bool, dir string) []check {
	if !installed {
		return []check{{"menu bar", info, "Hammerspoon not installed", ""}}
	}
	spoon := filepath.Join(dir, "Spoons", install.SpoonName+".spoon", "init.lua")
	initLua, _ := os.ReadFile(filepath.Join(dir, "init.lua"))
	switch {
	case !fileExists(spoon):
		return []check{{"menu bar", warn, "indicator Spoon not installed", "zmk-vim-mode install --hammerspoon"}}
	case !install.SpoonLoaded(initLua):
		return []check{{"menu bar", warn, "Spoon installed, but init.lua never starts it", "zmk-vim-mode install --hammerspoon"}}
	}
	return []check{{"menu bar", ok, install.SpoonName + " Spoon installed and started from init.lua", ""}}
}

// omarchyChecks looks for the widget plugin and for its id in the bar layout.
func omarchyChecks(home string) []check {
	shellJSON, err := os.ReadFile(filepath.Join(install.OmarchyConfigDir(home), "shell.json"))
	if err != nil {
		return []check{{"omarchy bar", info, "no Omarchy Quattro shell.json", ""}}
	}
	inBar, _ := install.OmarchyWidgetEnabled(shellJSON)
	switch {
	case !fileExists(filepath.Join(install.OmarchyPluginDir(home), "manifest.json")):
		return []check{{"omarchy bar", warn, "widget plugin not installed", "zmk-vim-mode install --omarchy"}}
	case !inBar:
		return []check{{"omarchy bar", warn, "widget installed, but shell.json does not put it in the bar",
			"zmk-vim-mode install --omarchy, or: omarchy plugin enable " + install.OmarchyPluginID}}
	}
	return []check{{"omarchy bar", ok, install.OmarchyPluginID + " installed and in the bar", ""}}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}
