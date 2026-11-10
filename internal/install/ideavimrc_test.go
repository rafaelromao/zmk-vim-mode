package install

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyIdeaVimRCAppendsToExistingFile(t *testing.T) {
	out, conflict, err := ApplyIdeaVimRC("set number")
	if err != nil || conflict {
		t.Fatalf("err=%v conflict=%v", err, conflict)
	}
	if !strings.HasPrefix(out, "set number\n\n"+ideaVimRCBegin) || !strings.HasSuffix(out, ideaVimRCEnd+"\n") {
		t.Fatalf("block not appended after a blank line:\n%s", out)
	}
}

func TestApplyIdeaVimRCEmptyFileIsJustTheBlock(t *testing.T) {
	out, _, _ := ApplyIdeaVimRC("")
	if out != ideaVimRCBlock {
		t.Fatalf("got %q", out)
	}
}

func TestApplyIdeaVimRCIncludesInsertWordDelete(t *testing.T) {
	out, conflict, err := ApplyIdeaVimRC("")
	if err != nil || conflict {
		t.Fatalf("err=%v conflict=%v", err, conflict)
	}
	for _, mapping := range []string{"inoremap <C-BS> <C-W>", `inoremap <C-Del> <C-O>"_dw`} {
		if !strings.Contains(out, mapping+"\n") {
			t.Errorf("missing insert mapping %q in:\n%s", mapping, out)
		}
	}
}

func TestApplyIdeaVimRCPreservesInsertMappingOutsideManagedBlock(t *testing.T) {
	for _, mapping := range []string{"inoremap <C-BS> <C-U>\n", "imap <C-Del> <Del>\n"} {
		for _, src := range []string{mapping, mapping + ideaVimRCBlock, ideaVimRCBlock + mapping} {
			out, conflict, err := ApplyIdeaVimRC(src)
			if err != nil || !conflict || out != src {
				t.Errorf("%q: conflict=%v err=%v out=%q", src, conflict, err, out)
			}
		}
	}
}

func TestApplyIdeaVimRCIsIdempotent(t *testing.T) {
	once, _, _ := ApplyIdeaVimRC("set number\n")
	twice, conflict, err := ApplyIdeaVimRC(once)
	if err != nil || conflict || twice != once {
		t.Fatalf("second run changed the file (err=%v conflict=%v):\n%s", err, conflict, twice)
	}
}

func TestApplyIdeaVimRCReplacesStaleBlockInPlace(t *testing.T) {
	stale := "set number\n" + ideaVimRCBegin + "\nnnoremap <C-BS> old\n" + ideaVimRCEnd + "\nset relativenumber\n"
	out, conflict, err := ApplyIdeaVimRC(stale)
	if err != nil || conflict {
		t.Fatalf("err=%v conflict=%v", err, conflict)
	}
	want := "set number\n" + ideaVimRCBlock + "set relativenumber\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	if !strings.Contains(out, "inoremap <C-BS> <C-W>\n") {
		t.Fatal("stale block was not upgraded with insert word-delete")
	}
}

func TestApplyIdeaVimRCLeavesUsersOwnMappingAlone(t *testing.T) {
	for _, src := range []string{
		"nnoremap <C-BS> db\n",
		"  noremap <C-Del> x\n",
		"map <C-BS> db\n",
		"xnoremap <silent> <C-bs> d\n",
	} {
		out, conflict, err := ApplyIdeaVimRC(src)
		if err != nil || !conflict || out != src {
			t.Errorf("%q: conflict=%v err=%v out=%q", src, conflict, err, out)
		}
	}
}

func TestApplyIdeaVimRCIgnoresOtherKeysAndComments(t *testing.T) {
	_, conflict, _ := ApplyIdeaVimRC("\" nnoremap <C-BS> db\nnnoremap <C-S> :w<CR>\n")
	if conflict {
		t.Fatal("a comment or an unrelated key counted as a conflict")
	}
}

func TestApplyIdeaVimRCRefusesAnUnterminatedBlock(t *testing.T) {
	src := "set number\n" + ideaVimRCBegin + "\nnnoremap <C-BS> db\n"
	out, _, err := ApplyIdeaVimRC(src)
	if err == nil || out != src {
		t.Fatalf("err=%v out=%q", err, out)
	}
}

func TestInstallIdeaVimRCCreatesThenSettles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	var w bytes.Buffer
	if err := InstallIdeaVimRC(&w, home); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(home, ".ideavimrc"))
	if err != nil || string(b) != ideaVimRCBlock {
		t.Fatalf("err=%v content=%q", err, b)
	}
	if !strings.Contains(w.String(), "created") {
		t.Fatalf("output: %s", w.String())
	}
	w.Reset()
	if err := InstallIdeaVimRC(&w, home); err != nil || !strings.Contains(w.String(), "already has") {
		t.Fatalf("err=%v output: %s", err, w.String())
	}
}

func TestInstallIdeaVimRCPatchesTheFileIdeaVimReads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	xdg := filepath.Join(home, ".config", "ideavim", "ideavimrc")
	if err := os.MkdirAll(filepath.Dir(xdg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdg, []byte("set number\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := InstallIdeaVimRC(&bytes.Buffer{}, home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".ideavimrc")); err == nil {
		t.Fatal("created ~/.ideavimrc, which would shadow the file IdeaVim already uses")
	}
	b, _ := os.ReadFile(xdg)
	if !strings.HasPrefix(string(b), "set number\n") || !strings.Contains(string(b), ideaVimRCBegin) {
		t.Fatalf("not patched:\n%s", b)
	}
	if st, _ := os.Stat(xdg); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode changed to %v", st.Mode().Perm())
	}
}

func TestInstallIdeaVimRCFollowsASymlink(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "")
	real := filepath.Join(t.TempDir(), "ideavimrc")
	if err := os.WriteFile(real, []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, ".ideavimrc")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("no symlinks here")
	}
	if err := InstallIdeaVimRC(&bytes.Buffer{}, home); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Lstat(link); st.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symlink was replaced by a regular file")
	}
	if b, _ := os.ReadFile(real); !strings.Contains(string(b), ideaVimRCBegin) {
		t.Fatalf("target not patched:\n%s", b)
	}
}
