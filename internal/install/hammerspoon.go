package install

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	hammerspoonbar "github.com/rafaelromao/zmk-vim-mode/bars/hammerspoon"
)

// SpoonName is the menu bar indicator's Spoon: Spoons/ZmkVimMode.spoon in
// Hammerspoon's config directory, started with hs.loadSpoon("ZmkVimMode").
const SpoonName = "ZmkVimMode"

// spoonLoader is what gets appended to init.lua to start the Spoon. It is
// guarded so it can never cost the user the rest of their config: a Spoon that
// has been removed is skipped, and one that fails to load is reported in the
// console instead of aborting init.lua halfway.
const spoonLoader = `
-- added by zmk-vim-mode: vim mode indicator in the menu bar
if hs.fs.attributes(hs.configdir .. "/Spoons/ZmkVimMode.spoon/init.lua") then
  local ok, err = pcall(function() hs.loadSpoon("ZmkVimMode"):start() end)
  if not ok then print("ZmkVimMode: " .. tostring(err)) end
end
`

// HammerspoonInstalled reports whether Hammerspoon.app is where Homebrew or a
// drag-and-drop install puts it.
func HammerspoonInstalled(home string) bool {
	for _, dir := range []string{"/Applications", filepath.Join(home, "Applications")} {
		if dirExists(filepath.Join(dir, "Hammerspoon.app")) {
			return true
		}
	}
	return false
}

// HammerspoonConfigFile is the MJConfigFile preference, the init.lua
// Hammerspoon was pointed at instead of its default; "" when unset.
func HammerspoonConfigFile() string {
	out, err := exec.Command("defaults", "read", "org.hammerspoon.Hammerspoon", "MJConfigFile").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// HammerspoonConfigDir is where Hammerspoon reads init.lua and looks for
// Spoons: ~/.hammerspoon, or the directory of configFile (the MJConfigFile
// preference) when that is set.
func HammerspoonConfigDir(home, configFile string) string {
	switch {
	case configFile == "":
		return filepath.Join(home, ".hammerspoon")
	case configFile == "~", strings.HasPrefix(configFile, "~/"):
		configFile = filepath.Join(home, configFile[1:])
	}
	return filepath.Dir(configFile)
}

// SpoonLoaded reports whether an init.lua does anything with the Spoon. What
// it does is the user's business: a file that mentions it is left alone.
func SpoonLoaded(initLua []byte) bool {
	return bytes.Contains(initLua, []byte(SpoonName))
}

// installSpoon writes the Spoon into spoonsDir with bin as its binary, and
// reports whether anything changed. A Spoon directory that is a symlink is a
// development link into a checkout and is left alone (linked): writing the
// binary's path into it would edit the checkout.
func installSpoon(spoonsDir, bin string) (copied, linked bool, err error) {
	dir := filepath.Join(spoonsDir, SpoonName+".spoon")
	if isSymlink(dir) {
		return false, true, nil
	}
	src, err := fs.Sub(hammerspoonbar.Files, SpoonName+".spoon")
	if err != nil {
		return false, false, err
	}
	copied, err = writeAssets(src, dir, bin)
	return copied, false, err
}

// ensureSpoonLoader makes init.lua start the Spoon, and reports whether it had
// to. The block is appended, so an init.lua that is a symlink is written
// through, never replaced by a copy.
func ensureSpoonLoader(initPath string) (bool, error) {
	old, err := os.ReadFile(initPath)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if SpoonLoaded(old) {
		return false, nil
	}
	block := spoonLoader
	switch {
	case len(old) == 0:
		block = strings.TrimPrefix(block, "\n")
	case old[len(old)-1] != '\n':
		block = "\n" + block
	}
	if err := os.MkdirAll(filepath.Dir(initPath), 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(initPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false, err
	}
	_, werr := f.WriteString(block)
	cerr := f.Close()
	if werr != nil {
		return false, werr
	}
	return true, cerr
}

// reloadHammerspoon asks a running Hammerspoon to reload its config so the
// Spoon starts now. That goes through the hs CLI, which needs hs.ipc loaded
// in init.lua; without it this says what to do instead. The reload is
// deferred a moment so the CLI gets its answer before the reload tears down
// the port it is talking to, and the timer is kept in a global so it is not
// collected before it fires. Never -A: that would launch Hammerspoon.
func reloadHammerspoon(w io.Writer) {
	if exec.Command("pgrep", "-x", "Hammerspoon").Run() != nil {
		fmt.Fprintln(w, "hammerspoon: not running; the indicator starts with it")
		return
	}
	hs, err := exec.LookPath("hs")
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = exec.CommandContext(ctx, hs, "-q", "-t", "2", "-c",
			"zmkVimModeReload = hs.timer.doAfter(0.2, hs.reload)").Run()
	}
	if err != nil {
		fmt.Fprintln(w, "hammerspoon: reload its config to start the indicator (menu bar icon → Reload Config)")
		return
	}
	fmt.Fprintln(w, "hammerspoon: reloaded its config")
}

// InstallHammerspoon installs the menu bar indicator: the Spoon, with this
// binary's path written into it, and the lines in init.lua that start it.
func InstallHammerspoon(w io.Writer, exe string) error {
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(w, "hammerspoon: macOS only; nothing to do")
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if !HammerspoonInstalled(home) {
		fmt.Fprintln(w, "hammerspoon: not found; install it (brew install --cask hammerspoon) and run this again")
		return nil
	}
	dir := HammerspoonConfigDir(home, HammerspoonConfigFile())
	spoon := filepath.Join(dir, "Spoons", SpoonName+".spoon")
	copied, linked, err := installSpoon(filepath.Join(dir, "Spoons"), exe)
	if err != nil {
		return err
	}
	switch {
	case linked:
		fmt.Fprintf(w, "hammerspoon: %s is a symlink (a development checkout); left alone\n", TrimHome(spoon))
	case copied:
		fmt.Fprintf(w, "hammerspoon: installed %s\n", TrimHome(spoon))
	default:
		fmt.Fprintf(w, "hammerspoon: %s is up to date\n", TrimHome(spoon))
	}
	initPath := filepath.Join(dir, "init.lua")
	added, err := ensureSpoonLoader(initPath)
	if err != nil {
		return err
	}
	if added {
		fmt.Fprintf(w, "hammerspoon: added the lines that start it to %s%s\n", TrimHome(initPath), linkNote(initPath))
	} else {
		fmt.Fprintf(w, "hammerspoon: %s already loads it\n", TrimHome(initPath))
	}
	if copied || added {
		reloadHammerspoon(w)
		fmt.Fprintln(w, "the glyph needs a Nerd Font, any of them (e.g. brew install --cask font-hack-nerd-font);")
		fmt.Fprintln(w, "without one the item shows the mode alone.")
	}
	return nil
}
