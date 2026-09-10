// Package atspi turns accessibility-bus focus events into "is keyboard focus in
// the text editor?" facts. The accessibility bus is the only observer that
// sees a quick input opened with the mouse: no VSCode API fires for it and
// the window title does not change.
package atspi

import (
	"sort"
	"strings"
)

// Focused describes the accessible object that just took focus.
type Focused struct {
	Role  string            // AT-SPI role name, e.g. "entry", "push button", "terminal"
	Name  string            // accessible name (aria-label / label)
	Attrs map[string]string // object attributes; Chromium sets "tag", "class", "roledescription"
	// AncestorClasses holds the "class" attribute of each exposed ancestor,
	// nearest first. Chromium hides generic containers, so this is a bonus,
	// not something the rules depend on.
	AncestorClasses []string
}

// Verdict is what the daemon learns from one focus event.
type Verdict struct {
	Ignore bool   // not a widget: the window, a frame, a container
	Editor bool   // keyboard focus is in the code editor
	Detail string // human-readable, for logs and `status`
}

// Roles that are containers or the window itself: focus "landing" on them is
// not a widget change.
var ignoredRoles = map[string]bool{
	"frame": true, "window": true, "application": true, "document web": true, "document frame": true,
	"panel": true, "filler": true, "section": true, "scroll pane": true, "viewport": true,
	"root pane": true, "layered pane": true, "glass pane": true, "invalid": true, "unknown": true,
	"redundant object": true, "embedded": true, "": true,
}

// Monaco editors that are not *the* editor: their accessible names, as VSCode
// labels them (English; a localized UI extends this through --atspi-widget).
var monacoWidgetNames = []string{
	"find", "replace", "source control", "debug console", "search", "rename", "settings",
	"keybindings", "extensions", "chat", "comment", "commit message", "terminal",
}

// Ancestor classes that mark a Monaco editor as a widget, whatever its name.
var monacoWidgetAncestors = []string{
	"find-widget", "quick-input-widget", "rename-box", "scm-editor", "settings-editor",
	"search-view", "debug-console", "repl", "interactive-input", "chat-input", "comments",
}

// ClassifyVSCode applies the rules for VSCode's DOM as Chromium exposes it.
// Anything that is not recognisably the code editor is "other": raw is the
// safe failure. Monaco text areas default to "editor", so an unrecognised
// Monaco-based widget degrades to today's behaviour rather than to "vim never
// comes on".
func ClassifyVSCode(f Focused) Verdict {
	role := strings.ToLower(strings.TrimSpace(f.Role))
	if ignoredRoles[role] {
		if role == "" {
			role = "no role"
		}
		return Verdict{Ignore: true, Detail: role}
	}
	tag := strings.ToLower(f.Attrs["tag"])
	classes := tokens(f.Attrs["class"])
	name := strings.ToLower(strings.TrimSpace(f.Name))

	if classes["inputarea"] || strings.EqualFold(f.Attrs["roledescription"], "editor") {
		for _, anc := range f.AncestorClasses {
			for _, marker := range monacoWidgetAncestors {
				if tokens(anc)[marker] {
					return Verdict{Detail: "monaco widget: " + marker}
				}
			}
		}
		for _, w := range monacoWidgetNames {
			if strings.HasPrefix(name, w) {
				return Verdict{Detail: "monaco widget: " + f.Name}
			}
		}
		return Verdict{Editor: true, Detail: "monaco editor"}
	}
	if classes["xterm-helper-textarea"] || role == "terminal" {
		return Verdict{Detail: "terminal"}
	}
	if tag == "input" || tag == "textarea" {
		return Verdict{Detail: describe(tag, classes, f.Name)}
	}
	return Verdict{Detail: describe(role, classes, f.Name)}
}

func describe(what string, classes map[string]bool, name string) string {
	var cs []string
	for c := range classes {
		cs = append(cs, c)
	}
	sort.Strings(cs)
	if len(cs) > 3 {
		cs = cs[:3]
	}
	s := what
	if len(cs) > 0 {
		s += "." + strings.Join(cs, ".")
	}
	if name != "" {
		s += " (" + truncate(name, 60) + ")"
	}
	return s
}

func tokens(class string) map[string]bool {
	out := map[string]bool{}
	for _, t := range strings.Fields(class) {
		out[strings.ToLower(t)] = true
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
