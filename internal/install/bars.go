package install

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The status bar indicators (the Hammerspoon Spoon in hammerspoon.go, the
// Omarchy widget in omarchy.go) are files this package writes out from the
// binary, with the path of the binary doing the install written into them.

// binaryPlaceholder is the quoted string the bar assets carry where the
// binary's path goes. The assets test for an absolute path rather than
// comparing with this string: writing the path replaces every quoted copy of
// the placeholder, a comparison's included.
const binaryPlaceholder = `"__ZMK_VIM_MODE_BIN__"`

// withBinary writes bin into an asset in place of the placeholder, as a
// double-quoted string both Lua and QML read the same way.
func withBinary(b []byte, bin string) []byte {
	quoted := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(bin) + `"`
	return bytes.ReplaceAll(b, []byte(binaryPlaceholder), []byte(quoted))
}

// writeAssets copies the tree src into dir with bin written into every file,
// and reports whether anything changed. Files are written as regular files,
// replacing a symlink if one is in the way, and a file that already has the
// right content is not touched.
func writeAssets(src fs.FS, dir, bin string) (bool, error) {
	changed := false
	err := fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		b = withBinary(b, bin)
		if isSymlink(target) {
			if err := os.Remove(target); err != nil {
				return err
			}
		} else if old, err := os.ReadFile(target); err == nil && bytes.Equal(old, b) {
			return nil
		}
		if err := os.WriteFile(target, b, 0o644); err != nil {
			return err
		}
		changed = true
		return nil
	})
	return changed, err
}

func isSymlink(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode()&fs.ModeSymlink != 0
}

// linkNote says where a symlinked config really lives, so an edit that lands
// in the file behind the link says which file that is.
func linkNote(p string) string {
	if !isSymlink(p) {
		return ""
	}
	target, err := filepath.EvalSymlinks(p)
	if err != nil {
		return ""
	}
	return " (→ " + TrimHome(target) + ")"
}

func haveCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
