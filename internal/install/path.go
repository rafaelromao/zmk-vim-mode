package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// A fresh macOS has no ~/.local/bin on PATH, and the only symptom is "command
// not found" from a command that installed perfectly. The service runs the
// binary by full path, so the daemon works either way; it is the CLI
// (`zmk-vim-mode doctor`, `zmk-vim-mode set`) that goes missing.

// pathMarker tags the line this package appends, so a second install can tell
// its own edit from one the user wrote by hand.
const pathMarker = "# added by zmk-vim-mode"

// profileFor returns the shell profile a PATH edit belongs in, for the given
// login shell. An empty string means the shell is not one we know how to edit
// safely, and the caller should print advice instead of guessing.
//
// zsh reads ~/.zshrc for interactive shells, which is the one that matters for
// a command typed at a prompt. bash reads ~/.bashrc on Linux, but on macOS
// Terminal starts login shells, which read ~/.bash_profile instead.
func profileFor(shell, home string) string {
	switch filepath.Base(shell) {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, ".bash_profile")
		}
		return filepath.Join(home, ".bashrc")
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish")
	}
	return ""
}

// underHome rewrites dir through $HOME when it lives there, and returns it
// unchanged otherwise. Both sides are resolved first: the caller hands us a
// directory taken from the running binary, which has been through
// EvalSymlinks, while $HOME has not — on a machine where the home directory
// sits behind a symlink, comparing them raw silently writes an absolute path.
func underHome(dir, home string) string {
	for _, h := range []string{home, resolve(home)} {
		if rel, err := filepath.Rel(h, resolve(dir)); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("$HOME", rel)
		}
	}
	return dir
}

func resolve(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// pathSnippet is what gets appended to profile to put dir on PATH. dir is
// written through $HOME rather than ~, which does not expand inside the double
// quotes these lines need.
func pathSnippet(profile, dir, home string) string {
	d := underHome(dir, home)
	if filepath.Base(profile) == "config.fish" {
		return fmt.Sprintf("\n%s\nfish_add_path %s\n", pathMarker, d)
	}
	return fmt.Sprintf("\n%s\nexport PATH=\"%s:$PATH\"\n", pathMarker, d)
}

// onPath reports whether dir is already an entry of pathEnv. Entries are
// resolved before comparing, so a PATH written through a symlinked home still
// matches the directory the binary actually sits in — but an unexpanded "~"
// (what a profile leaves behind when it quotes a tilde) resolves to nothing
// and stays a non-match, which is the very problem this check exists for.
func onPath(pathEnv, dir string) bool {
	want := resolve(filepath.Clean(dir))
	for _, e := range filepath.SplitList(pathEnv) {
		if e != "" && resolve(filepath.Clean(e)) == want {
			return true
		}
	}
	return false
}

// displayDir names dir the way a person writes it, so messages say
// "~/.local/bin" rather than the resolved path behind it.
func displayDir(dir, home string) string {
	return strings.Replace(underHome(dir, home), "$HOME", "~", 1)
}

// EnsurePATH puts dir on PATH for future shells by appending to the login
// shell's profile. It cannot change the PATH of the shell that invoked it —
// no child process can — so it says what to do about the current one.
func EnsurePATH(w io.Writer, dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if onPath(os.Getenv("PATH"), dir) {
		fmt.Fprintf(w, "path       : %s is already on PATH\n", displayDir(dir, home))
		return nil
	}
	profile := profileFor(os.Getenv("SHELL"), home)
	if profile == "" {
		fmt.Fprintf(w, "path       : %s is not on PATH, and $SHELL (%s) is not one I know how to edit.\n",
			displayDir(dir, home), os.Getenv("SHELL"))
		fmt.Fprintf(w, "             add this to your shell profile yourself:  export PATH=\"%s:$PATH\"\n",
			underHome(dir, home))
		return nil
	}
	if b, err := os.ReadFile(profile); err == nil && strings.Contains(string(b), pathMarker) {
		// The line is there but PATH does not have it: the running shell
		// predates the edit, or something later in the profile overwrote PATH.
		fmt.Fprintf(w, "path       : %s already has the PATH line, but this shell has not read it\n", TrimHome(profile))
		fmt.Fprintf(w, "             start a new shell (or: exec %s)\n", filepath.Base(os.Getenv("SHELL")))
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(profile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(pathSnippet(profile, dir, home)); err != nil {
		return err
	}
	fmt.Fprintf(w, "path       : added %s to PATH in %s\n", displayDir(dir, home), TrimHome(profile))
	fmt.Fprintf(w, "             start a new shell to pick it up (or: exec %s)\n", filepath.Base(os.Getenv("SHELL")))
	return nil
}
