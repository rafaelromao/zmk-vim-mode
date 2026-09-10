package atspi

import "testing"

func TestClassifyVSCode(t *testing.T) {
	monaco := map[string]string{"tag": "textarea", "class": "inputarea monaco-mouse-cursor-text", "roledescription": "editor"}
	cases := []struct {
		name   string
		f      Focused
		editor bool
		ignore bool
	}{
		{"code editor, accessibility off label",
			Focused{Role: "entry", Name: "The editor is not accessible at this time. To enable screen reader optimized mode, use Shift+Alt+F1", Attrs: monaco}, true, false},
		{"code editor, no name", Focused{Role: "entry", Attrs: monaco}, true, false},
		{"code editor as seen on the box (native-edit-context div)",
			Focused{Role: "entry", Name: "The editor is not accessible at this time. To enable screen reader optimized mode, use Shift+Alt+F4",
				Attrs:           map[string]string{"tag": "div", "class": "native-edit-context", "roledescription": "editor", "xml-roles": "textbox"},
				AncestorClasses: []string{"overflow-guard", "monaco-editor no-user-select  showUnused showDeprecated vs-dark focused", "editor-instance"}}, true, false},
		{"palette list row as seen on the box",
			Focused{Role: "list item", Name: "Go to File, Go to File", Attrs: map[string]string{"tag": "div", "class": "monaco-list-row focused", "xml-roles": "option"}}, false, false},
		{"code editor, file label", Focused{Role: "entry", Name: "main.go", Attrs: monaco}, true, false},
		{"find widget input (Monaco) by name", Focused{Role: "entry", Name: "Find", Attrs: monaco}, false, false},
		{"replace input by name", Focused{Role: "entry", Name: "Replace", Attrs: monaco}, false, false},
		{"scm input by name", Focused{Role: "entry", Name: "Source Control Input", Attrs: monaco}, false, false},
		{"settings search by name", Focused{Role: "entry", Name: "Search settings", Attrs: monaco}, false, false},
		{"find widget by ancestor, unknown name",
			Focused{Role: "entry", Name: "whatever", Attrs: monaco, AncestorClasses: []string{"monaco-inputbox", "find-widget visible"}}, false, false},
		{"quick input box", Focused{Role: "entry", Name: "Type the name of a command to run.", Attrs: map[string]string{"tag": "input", "class": "input"}}, false, false},
		{"no attributes exposed: editor recognised by its label",
			Focused{Role: "entry", Name: "The editor is not accessible at this time. To enable screen reader optimized mode, use Shift+Alt+F1"}, true, false},
		{"no attributes exposed: 'Editor content' label", Focused{Role: "text", Name: "Editor content;Press Alt+F1 for Accessibility Options."}, true, false},
		{"no attributes exposed: palette by label", Focused{Role: "entry", Name: "Type the name of a command to run."}, false, false},
		{"no attributes exposed: terminal by label", Focused{Role: "entry", Name: "Terminal input"}, false, false},
		{"no attributes exposed: unlabeled text box stays other", Focused{Role: "entry"}, false, false},
		{"rename input", Focused{Role: "entry", Name: "Rename input. Type new name and press Enter to commit.", Attrs: map[string]string{"tag": "input", "class": "rename-input"}}, false, false},
		{"terminal helper textarea", Focused{Role: "entry", Attrs: map[string]string{"tag": "textarea", "class": "xterm-helper-textarea"}}, false, false},
		{"terminal role", Focused{Role: "terminal", Name: "Terminal 1, zsh"}, false, false},
		{"explorer tree item", Focused{Role: "tree item", Name: "main.go"}, false, false},
		{"a tab", Focused{Role: "page tab", Name: "main.go, tab"}, false, false},
		{"a button", Focused{Role: "push button", Name: "Run"}, false, false},
		{"the window itself", Focused{Role: "frame", Name: "main.go — zmk [Text Editor]"}, false, true},
		{"document web container", Focused{Role: "document web"}, false, true},
		{"empty role", Focused{}, false, true},
	}
	for _, c := range cases {
		v := ClassifyVSCode(c.f)
		if v.Ignore != c.ignore || v.Editor != c.editor {
			t.Errorf("%s: got editor=%v ignore=%v (%s); want editor=%v ignore=%v", c.name, v.Editor, v.Ignore, v.Detail, c.editor, c.ignore)
		}
		if v.Detail == "" {
			t.Errorf("%s: empty detail", c.name)
		}
	}
	if d := ClassifyVSCode(Focused{Role: "entry", Name: "Type the name of a command to run.", Attrs: map[string]string{"tag": "input", "class": "input"}}).Detail; d != "input.input (Type the name of a command to run.)" {
		t.Errorf("detail: %q", d)
	}
}
