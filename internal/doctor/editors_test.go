package doctor

import "testing"

func TestWindowTitleMarker(t *testing.T) {
	cases := []struct {
		name     string
		settings string
		title    string
		good     bool
	}{
		{"recommended value",
			`{ "editor.fontSize": 14, "window.title": "${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]" }`,
			"${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]", true},
		{"marker not at the end",
			`{"window.title": "[${focusedView}] ${activeEditorShort"}`, "[${focusedView}] ${activeEditorShort", false},
		{"no marker", `{"window.title": "${activeEditorShort}"}`, "${activeEditorShort}", false},
		{"not set", `{"editor.fontSize": 14}`, "", false},
		{"comments and trailing comma survive",
			"{\n  // title\n  \"window.title\": \"x [${focusedView}]\",\n}\n", "x [${focusedView}]", true},
	}
	for _, c := range cases {
		title, good := windowTitleMarker(c.settings)
		if title != c.title || good != c.good {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, title, good, c.title, c.good)
		}
	}
}

func TestObsidianVaults(t *testing.T) {
	cfg := []byte(`{"vaults":{"a1":{"path":"/home/u/notes","ts":1},"b2":{"path":"/home/u/work","ts":2,"open":true}}}`)
	got := obsidianVaults(cfg)
	if len(got) != 2 || got[0] != "/home/u/notes" || got[1] != "/home/u/work" {
		t.Fatalf("vaults: %v", got)
	}
	if obsidianVaults([]byte("not json")) != nil {
		t.Fatal("garbage must yield no vaults")
	}
}

func TestCommunityPluginEnabled(t *testing.T) {
	list := []byte(`["obsidian-vimrc-support","zmk-vim-mode"]`)
	if !communityPluginEnabled(list, "zmk-vim-mode") {
		t.Fatal("listed plugin must be enabled")
	}
	if communityPluginEnabled(list, "other") || communityPluginEnabled(nil, "zmk-vim-mode") {
		t.Fatal("unlisted or missing list must be disabled")
	}
}

func TestHasExtension(t *testing.T) {
	names := []string{"asvetliakov.vscode-neovim-1.18.20", "rafaelromao.zmk-vim-mode-0.1.0", "extensions.json"}
	if !hasExtension(names, "asvetliakov.vscode-neovim-") || !hasExtension(names, "rafaelromao.zmk-vim-mode-") {
		t.Fatal("installed extensions must be found")
	}
	if hasExtension(names, "vscodevim.vim-") {
		t.Fatal("absent extension must not be found")
	}
}
