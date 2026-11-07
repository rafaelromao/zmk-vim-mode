package install

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The markers fence the block this package owns inside the user's ideavimrc,
// so a second install replaces it in place instead of stacking another copy.
const (
	ideaVimRCBegin = `" >>> zmk-vim-mode (managed by "zmk-vim-mode install --intellij"; edits in here are overwritten)`
	ideaVimRCEnd   = `" <<< zmk-vim-mode`
)

// ideaVimRCBlock gives Ctrl+Backspace and Ctrl+Delete a meaning in normal and
// visual mode. IdeaVim claims both keys in every mode (VimShortcutKeyAction's
// VIM_ONLY_EDITOR_KEYS) and sethandler cannot hand them back to the IDE, but
// it has no normal-mode command for them, so the keyboard's TEXT layer, which
// sends them for word-delete, did nothing. The black-hole register keeps the
// deleted text out of the yank register, as the IDE's own word-delete does.
const ideaVimRCBlock = ideaVimRCBegin + `
" IdeaVim owns <C-BS> and <C-Del> in every mode but maps nothing to them in
" normal mode; the keyboard's TEXT layer sends them for word-delete.
nnoremap <C-BS>  "_db
nnoremap <C-Del> "_dw
xnoremap <C-BS>  "_d
xnoremap <C-Del> "_d
` + ideaVimRCEnd + "\n"

// userMapsWordDelete matches a mapping of either key outside our block. The
// user's own wins: later definitions override earlier ones, and appending ours
// would silently replace theirs.
var userMapsWordDelete = regexp.MustCompile(`(?mi)^\s*[nvxo]?(?:nore)?map\s+(?:<[a-z]+>\s+)*<C-(?:BS|Del)>`)

// ApplyIdeaVimRC returns src with the managed block present and current. The
// block is replaced where it already is, and appended otherwise. When the user
// has mapped either key themselves the file is returned unchanged with
// conflict set, and a block with a missing end marker is an error: guessing
// where it stops could eat the user's lines.
func ApplyIdeaVimRC(src string) (out string, conflict bool, err error) {
	begin := strings.Index(src, `" >>> zmk-vim-mode`)
	if begin < 0 {
		if userMapsWordDelete.MatchString(src) {
			return src, true, nil
		}
		if src != "" && !strings.HasSuffix(src, "\n") {
			src += "\n"
		}
		if src != "" {
			src += "\n"
		}
		return src + ideaVimRCBlock, false, nil
	}
	endRel := strings.Index(src[begin:], ideaVimRCEnd)
	if endRel < 0 {
		return src, false, fmt.Errorf("found %q without its closing %q", `" >>> zmk-vim-mode`, ideaVimRCEnd)
	}
	end := begin + endRel + len(ideaVimRCEnd)
	if nl := strings.IndexByte(src[end:], '\n'); nl >= 0 {
		end += nl + 1
	} else {
		end = len(src)
	}
	before, after := src[:begin], src[end:]
	if userMapsWordDelete.MatchString(before + after) {
		return src, true, nil
	}
	return before + ideaVimRCBlock + after, false, nil
}

// ideaVimRCPath is the file IdeaVim will read. It takes the first of its
// search locations that exists, so patching any other would be ignored; when
// none does, the classic ~/.ideavimrc is created.
func ideaVimRCPath(home string) string {
	candidates := []string{
		filepath.Join(home, ".ideavimrc"),
		filepath.Join(home, "_ideavimrc"),
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(x) {
		candidates = append(candidates, filepath.Join(x, "ideavim", "ideavimrc"))
	}
	candidates = append(candidates, filepath.Join(home, ".config", "ideavim", "ideavimrc"))
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return candidates[0]
}

// InstallIdeaVimRC creates or patches the user's ideavimrc. Every IDE shares
// the one file, so it runs once however many IDEs were installed.
func InstallIdeaVimRC(w io.Writer, home string) error {
	p := ideaVimRCPath(home)
	old, err := os.ReadFile(p)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	out, conflict, err := ApplyIdeaVimRC(string(old))
	switch {
	case err != nil:
		fmt.Fprintf(w, "ideavimrc  : %s — left alone: %v\n", TrimHome(p), err)
		return nil
	case conflict:
		fmt.Fprintf(w, "ideavimrc  : %s — left alone: it already maps <C-BS> or <C-Del>\n", TrimHome(p))
		return nil
	case existed && out == string(old):
		fmt.Fprintf(w, "ideavimrc  : %s already has the Ctrl+Backspace / Ctrl+Delete mappings\n", TrimHome(p))
		return nil
	}
	// WriteFile follows a symlink, which is what a dotfiles-managed rc wants.
	mode := os.FileMode(0o644)
	if st, err := os.Stat(p); err == nil {
		mode = st.Mode().Perm()
	}
	if err := os.WriteFile(p, []byte(out), mode); err != nil {
		return err
	}
	verb := "created"
	if existed {
		verb = "updated"
	}
	fmt.Fprintf(w, "ideavimrc  : %s %s — Ctrl+Backspace / Ctrl+Delete delete words in normal mode (`:source %s` or restart)\n",
		TrimHome(p), verb, TrimHome(p))
	return nil
}
