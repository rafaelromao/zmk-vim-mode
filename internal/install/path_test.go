package install

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProfileFor(t *testing.T) {
	home := "/home/u"
	bash := filepath.Join(home, ".bashrc")
	if runtime.GOOS == "darwin" {
		// Terminal.app starts login shells, which never read ~/.bashrc.
		bash = filepath.Join(home, ".bash_profile")
	}
	for _, c := range []struct{ shell, want string }{
		{"/bin/zsh", filepath.Join(home, ".zshrc")},
		{"/opt/homebrew/bin/bash", bash},
		{"/usr/local/bin/fish", filepath.Join(home, ".config", "fish", "config.fish")},
		{"/usr/bin/nu", ""},
		{"", ""},
	} {
		if got := profileFor(c.shell, home); got != c.want {
			t.Errorf("profileFor(%q) = %q, want %q", c.shell, got, c.want)
		}
	}
}

func TestPathSnippet(t *testing.T) {
	home := "/home/u"
	dir := filepath.Join(home, ".local", "bin")
	// $HOME, never ~: a tilde inside double quotes is not expanded, which is
	// how a PATH entry ends up naming a directory that does not exist.
	zsh := pathSnippet(filepath.Join(home, ".zshrc"), dir, home)
	if !strings.Contains(zsh, `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Errorf("zsh snippet = %q", zsh)
	}
	if strings.Contains(zsh, "~") {
		t.Errorf("snippet must not use a tilde: %q", zsh)
	}
	fish := pathSnippet(filepath.Join(home, ".config", "fish", "config.fish"), dir, home)
	if !strings.Contains(fish, "fish_add_path $HOME/.local/bin") {
		t.Errorf("fish snippet = %q", fish)
	}
	// A directory outside home stays absolute.
	if out := pathSnippet(filepath.Join(home, ".zshrc"), "/opt/zmk/bin", home); !strings.Contains(out, `"/opt/zmk/bin:$PATH"`) {
		t.Errorf("out-of-home snippet = %q", out)
	}
}

func TestOnPath(t *testing.T) {
	for _, c := range []struct {
		env, dir string
		want     bool
	}{
		{"/usr/bin:/home/u/.local/bin:/bin", "/home/u/.local/bin", true},
		{"/usr/bin:/home/u/.local/bin/:/bin", "/home/u/.local/bin", true},
		{"/usr/bin:/bin", "/home/u/.local/bin", false},
		// An unexpanded tilde names nothing, so it must not count as a match.
		{"/usr/bin:~/.local/bin", "/home/u/.local/bin", false},
		{"", "/home/u/.local/bin", false},
	} {
		if got := onPath(c.env, c.dir); got != c.want {
			t.Errorf("onPath(%q, %q) = %v, want %v", c.env, c.dir, got, c.want)
		}
	}
}

func TestEnsurePATHAppendsOnceAndSkipsWhenPresent(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".local", "bin")
	profile := filepath.Join(home, ".zshrc")
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/bin/zsh")
	t.Setenv("PATH", "/usr/bin:/bin")

	var out bytes.Buffer
	if err := EnsurePATH(&out, dir); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("the profile should have been created: %v", err)
	}
	if !strings.Contains(string(first), `export PATH="$HOME/.local/bin:$PATH"`) {
		t.Fatalf("profile = %q", first)
	}

	// A second install must not stack another copy of the same line.
	out.Reset()
	if err := EnsurePATH(&out, dir); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != string(first) {
		t.Fatalf("the profile was edited twice:\n%s", second)
	}
	if !strings.Contains(out.String(), "already has the PATH line") {
		t.Errorf("output = %q", out.String())
	}

	// Once PATH really has it, the profile is left alone entirely.
	t.Setenv("PATH", "/usr/bin:"+dir)
	out.Reset()
	if err := EnsurePATH(&out, dir); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "already on PATH") {
		t.Errorf("output = %q", out.String())
	}
}

func TestEnsurePATHUnknownShellDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", "/usr/bin/nu")
	t.Setenv("PATH", "/usr/bin:/bin")
	var out bytes.Buffer
	if err := EnsurePATH(&out, filepath.Join(home, ".local", "bin")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("an unknown shell must not have its config guessed at: %v", entries)
	}
	if !strings.Contains(out.String(), "not one I know how to edit") {
		t.Errorf("output = %q", out.String())
	}
}
