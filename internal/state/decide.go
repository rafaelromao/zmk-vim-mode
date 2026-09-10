package state

import (
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rafaelromao/zmk-vim-mode/internal/focus"
)

// Focus is the tri-state focus a client reported. Unknown ranks above an
// explicit No so a setup without terminal focus events degrades to
// "most recently active client wins".
type Focus int8

const (
	FocusUnknown Focus = 0
	FocusYes     Focus = 1
	FocusNo      Focus = -1
)

// Client is one connected editor client.
type Client struct {
	ID   uint64
	Kind string // nvim | vscode | obsidian | intellij
	// App is the host application kind the client lives in: "" for a
	// terminal-hosted (or GUI) Neovim, otherwise vscode|obsidian|intellij.
	App      string
	PID      int
	Mode     Mode
	Focus    Focus
	FocusSeq uint64 // store sequence number of the last focus message
	EventSeq uint64 // store sequence number of the last message of any kind
	Nested   bool   // nvim inside another nvim's :terminal
	// Deadline, when non-zero, is when this client's last report expires.
	// An expired report is no opinion (see None): the VSCode companion sends
	// its quick-input "raw" hint with a TTL so a missed close never traps the
	// keyboard in raw.
	Deadline  time.Time
	LastEvent time.Time
}

// Expired reports whether the client's timed report has lapsed.
func (c Client) Expired(now time.Time) bool {
	return !c.Deadline.IsZero() && now.After(c.Deadline)
}

// Opinion reports whether the client's current report can drive a decision.
func (c Client) Opinion(now time.Time) bool {
	return c.Mode.Opinion() && !c.Expired(now)
}

// Override is a manual `set` that replaces the automatic decision.
type Override struct {
	Mode   Mode
	Until  time.Time // zero = no TTL
	Sticky bool      // survive frontmost-app changes
}

// Active reports whether the override still applies at now.
func (o *Override) Active(now time.Time) bool {
	return o != nil && (o.Until.IsZero() || now.Before(o.Until))
}

// AppRule maps window classes of a vim-enabled application to the client
// kind that may report for it. Kind "" means "plain legacy app, no client ever".
type AppRule struct {
	Kind    string
	Classes []string
	// TitleView, when set, extracts the focused view's name from the window
	// title (capture group 1). A name outside EditorViews means a tool window
	// (terminal, sidebar, panel) has focus inside the app → Raw, whatever the
	// clients say: they cannot see focus leave the text editor, the title can.
	// A title without the marker is ignored (setting not applied).
	TitleView *regexp.Regexp
	// EditorViews are the marker values that mean "the text editor has focus".
	// Compared case-insensitively. VSCode reports "Text Editor" (older builds:
	// an empty string).
	EditorViews []string
}

// VSCodeWindowTitle is the `window.title` setting that makes VSCode publish
// the focused view in its title, where VSCodeFocusedView can read it.
const VSCodeWindowTitle = "${dirty}${activeEditorShort}${separator}${rootName} [${focusedView}]"

// VSCodeFocusedView matches the "[View Name]" marker at the end of the title
// (possibly empty).
var VSCodeFocusedView = regexp.MustCompile(`\[([^\[\]]*)\]\s*$`)

// VSCodeEditorViews are the marker values VSCode uses while the text editor
// has focus.
var VSCodeEditorViews = []string{"", "Text Editor", "Editor"}

// Rules is the app classification configuration.
type Rules struct {
	// TerminalApps host Neovim in a terminal emulator.
	TerminalApps []string
	// GUINvimApps are Neovim GUIs (neovide, nvim-qt).
	GUINvimApps []string
	// LegacyApps are vim-enabled editors without an intrinsic mode feed.
	LegacyApps []AppRule
	// TitleLegacy: substrings that, when found in a terminal window title with
	// no connected client, mean a plugin-less nvim (SSH) → Legacy. Empty disables.
	TitleLegacy []string
}

// DefaultRules returns the built-in classification for Hyprland app_ids and
// macOS bundle identifiers.
func DefaultRules() Rules {
	return Rules{
		TerminalApps: []string{
			"com.mitchellh.ghostty", "ghostty", "kitty", "org.wezfurlong.wezterm", "wezterm",
			"alacritty", "foot", "footclient", "org.omarchy.nvim", "xterm", "konsole", "org.kde.konsole",
			"gnome-terminal", "org.gnome.terminal", "com.googlecode.iterm2", "com.apple.terminal",
			"net.kovidgoyal.kitty", "com.github.wez.wezterm", "org.alacritty",
		},
		GUINvimApps: []string{"neovide", "com.neovide.neovide", "nvim-qt"},
		LegacyApps: []AppRule{
			{Kind: "vscode", Classes: []string{"code", "code-oss", "code-url-handler", "com.microsoft.vscode", "cursor", "codium"},
				TitleView: VSCodeFocusedView, EditorViews: VSCodeEditorViews},
			{Kind: "obsidian", Classes: []string{"obsidian", "md.obsidian"}},
			{Kind: "intellij", Classes: []string{"jetbrains-*", "com.jetbrains.*"}},
		},
		TitleLegacy: []string{"nvim", "neovim", "git rebase -i"},
	}
}

// Snapshot is the input to Decide.
type Snapshot struct {
	Frontmost focus.App
	Clients   []Client
	Override  *Override
	Now       time.Time
}

// Decision is the output of Decide.
type Decision struct {
	Mode     Mode
	Code     uint8
	Reason   string
	ClientID uint64 // 0 when no client drove the decision
}

func decision(m Mode, reason string, client uint64) Decision {
	return Decision{Mode: m, Code: m.Code(), Reason: reason, ClientID: client}
}

// Decide computes the desired keyboard state. It is a pure function.
//
// Priority: manual override → frontmost app with a connected client of the
// matching kind → legacy app → terminal/GUI-nvim app with the best-ranked
// Neovim client → title heuristic → OFF. An unknown frontmost (focus backend
// down) fails open and trusts the clients.
//
// Inside a vim-enabled app (VSCode, Obsidian) three sources combine, in order:
// the window title (a focused tool window → Raw), the app's own client saying
// Raw (quick input open, no text editor), then the best client with a real
// mode (Neovim embedded by vscode-neovim, or the app's own plugin). With none
// of those the app is Legacy and the keyboard infers modes by itself.
func Decide(r Rules, s Snapshot) Decision {
	now := s.Now
	if s.Override.Active(now) {
		return decision(s.Override.Mode, "override", 0)
	}
	front := s.Frontmost
	if !front.Known {
		if c, ok := best(opinionated(clientsOfApp(s.Clients, ""), now)); ok {
			return clientDecision(c, "frontmost unknown; trusting nvim client")
		}
		if c, ok := best(opinionated(s.Clients, now)); ok {
			return clientDecision(c, "frontmost unknown; trusting client")
		}
		return decision(Off, "frontmost unknown; no clients", 0)
	}
	if rule, ok := legacyRule(r, front.Class); ok {
		if rule.Kind != "" {
			if view, ok := toolWindow(rule, front.Title); ok {
				return decision(Raw, "tool window focused: "+view, 0)
			}
			appClients := clientsOfApp(s.Clients, rule.Kind)
			if g, ok := ownClient(appClients); ok && g.Mode == Raw && !g.Expired(now) {
				return decision(Raw, rule.Kind+" client raw", g.ID)
			}
			if c, ok := best(opinionated(appClients, now)); ok {
				return clientDecision(c, "client "+rule.Kind)
			}
		}
		return decision(Legacy, "legacy app "+front.Class, 0)
	}
	if matchClass(r.TerminalApps, front.Class) || matchClass(r.GUINvimApps, front.Class) {
		if c, ok := best(opinionated(clientsOfApp(s.Clients, ""), now)); ok {
			return clientDecision(c, "nvim client")
		}
		if titleMatches(r.TitleLegacy, front.Title) {
			return decision(Legacy, "title heuristic", 0)
		}
		return decision(Off, "terminal without nvim client", 0)
	}
	return decision(Off, "non-editor app "+front.Class, 0)
}

// toolWindow reads the focused view off the title and reports whether it is
// something other than the text editor.
func toolWindow(rule AppRule, title string) (string, bool) {
	if rule.TitleView == nil {
		return "", false
	}
	m := rule.TitleView.FindStringSubmatch(title)
	if m == nil || len(m) < 2 {
		return "", false
	}
	view := strings.TrimSpace(m[1])
	for _, e := range rule.EditorViews {
		if strings.EqualFold(view, strings.TrimSpace(e)) {
			return view, false
		}
	}
	return view, true
}

func clientDecision(c Client, reason string) Decision {
	return decision(c.Mode, reason, c.ID)
}

func clientsOfApp(cs []Client, app string) []Client {
	out := cs[:0:0]
	for _, c := range cs {
		if c.App == app {
			out = append(out, c)
		}
	}
	return out
}

// opinionated keeps the clients whose report can drive a decision: a real
// mode (not None) that has not expired.
func opinionated(cs []Client, now time.Time) []Client {
	out := cs[:0:0]
	for _, c := range cs {
		if c.Opinion(now) {
			out = append(out, c)
		}
	}
	return out
}

// ownClient returns the app's own client -- the one whose Kind is the app
// itself (the VSCode companion, the Obsidian plugin), as opposed to a Neovim
// embedded in it. The most recently heard one wins.
func ownClient(cs []Client) (Client, bool) {
	var own Client
	found := false
	for _, c := range cs {
		if c.Kind != c.App || c.App == "" {
			continue
		}
		if !found || c.EventSeq > own.EventSeq {
			own, found = c, true
		}
	}
	return own, found
}

// best ranks clients: explicit "not focused" clients are excluded whenever
// any other candidate exists; then focused=yes before unknown; then the most
// recent focus event; then nested (inner nvim in :terminal) before outer;
// then the most recent event of any kind.
func best(cs []Client) (Client, bool) {
	cands := make([]Client, 0, len(cs))
	for _, c := range cs {
		if c.Focus != FocusNo {
			cands = append(cands, c)
		}
	}
	if len(cands) == 0 {
		return Client{}, false
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Focus != b.Focus {
			return a.Focus > b.Focus
		}
		if a.FocusSeq != b.FocusSeq {
			return a.FocusSeq > b.FocusSeq
		}
		if a.Nested != b.Nested {
			return a.Nested
		}
		return a.EventSeq > b.EventSeq
	})
	return cands[0], true
}

func legacyRule(r Rules, class string) (AppRule, bool) {
	for _, rule := range r.LegacyApps {
		if matchClass(rule.Classes, class) {
			return rule, true
		}
	}
	return AppRule{}, false
}

// matchClass matches case-insensitively; patterns may contain '*' globs.
func matchClass(patterns []string, class string) bool {
	if class == "" {
		return false
	}
	c := strings.ToLower(class)
	for _, p := range patterns {
		p = strings.ToLower(p)
		if p == c {
			return true
		}
		if strings.ContainsAny(p, "*?[") {
			if ok, err := path.Match(p, c); err == nil && ok {
				return true
			}
		}
	}
	return false
}

func titleMatches(subs []string, title string) bool {
	if title == "" {
		return false
	}
	t := strings.ToLower(title)
	for _, s := range subs {
		if s != "" && strings.Contains(t, strings.ToLower(s)) {
			return true
		}
	}
	return false
}
