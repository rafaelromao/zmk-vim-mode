package state

import (
	"path"
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
	// Deadline, when non-zero, is when this client's mode lapses to Raw
	// (VSCode: keys stop reaching nvim while a tool window has focus).
	Deadline  time.Time
	LastEvent time.Time
}

// Expired reports whether the client's mode TTL has lapsed.
func (c Client) Expired(now time.Time) bool {
	return !c.Deadline.IsZero() && now.After(c.Deadline)
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
}

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
			{Kind: "vscode", Classes: []string{"code", "code-oss", "code-url-handler", "com.microsoft.vscode", "cursor", "codium"}},
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
func Decide(r Rules, s Snapshot) Decision {
	now := s.Now
	if s.Override.Active(now) {
		return decision(s.Override.Mode, "override", 0)
	}
	front := s.Frontmost
	if !front.Known {
		if c, ok := best(clientsOfApp(s.Clients, "")); ok {
			return clientDecision(c, now, "frontmost unknown; trusting nvim client")
		}
		if c, ok := best(s.Clients); ok {
			return clientDecision(c, now, "frontmost unknown; trusting client")
		}
		return decision(Off, "frontmost unknown; no clients", 0)
	}
	if rule, ok := legacyRule(r, front.Class); ok {
		if rule.Kind != "" {
			if c, ok := best(clientsOfApp(s.Clients, rule.Kind)); ok {
				return clientDecision(c, now, "client "+rule.Kind)
			}
		}
		return decision(Legacy, "legacy app "+front.Class, 0)
	}
	if matchClass(r.TerminalApps, front.Class) || matchClass(r.GUINvimApps, front.Class) {
		if c, ok := best(clientsOfApp(s.Clients, "")); ok {
			return clientDecision(c, now, "nvim client")
		}
		if titleMatches(r.TitleLegacy, front.Title) {
			return decision(Legacy, "title heuristic", 0)
		}
		return decision(Off, "terminal without nvim client", 0)
	}
	return decision(Off, "non-editor app "+front.Class, 0)
}

func clientDecision(c Client, now time.Time, reason string) Decision {
	if c.Expired(now) {
		return decision(Raw, reason+" (ttl lapsed)", c.ID)
	}
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
