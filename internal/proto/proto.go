// Package proto defines the newline-delimited JSON protocol spoken over the
// daemon's unix socket by editor clients (Neovim plugin, editor extensions) and
// by the CLI.
//
// Every message carries "t". "v" (protocol version) is required on hello and
// on CLI requests. Unknown message types and unknown fields are ignored.
package proto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Version is the protocol version this build speaks.
const Version = 1

// MaxLine is the maximum accepted length of one message line (bytes).
const MaxLine = 64 * 1024

// Message types.
const (
	// client → daemon
	THello = "hello"
	TMode  = "mode"
	TFocus = "focus"
	TBye   = "bye"
	// daemon → client
	TWelcome = "welcome"
	TResync  = "resync"
	TError   = "error"
	// CLI → daemon
	TSet     = "set"
	TStatus  = "status"
	TDevices = "devices"
	// daemon → CLI
	TOK = "ok"
)

// Error codes carried in Msg.Err.
const (
	ErrUnsupportedVersion = "unsupported_version"
	ErrBadRequest         = "bad_request"
)

// Msg is the union of all message shapes. Zero-valued fields are omitted on the wire.
type Msg struct {
	V int    `json:"v,omitempty"`
	T string `json:"t"`

	// Client → daemon.
	Client  string `json:"client,omitempty"` // nvim | vscode | obsidian | intellij
	App     string `json:"app,omitempty"`    // host application kind: "" (terminal-hosted nvim), vscode, obsidian, intellij
	PID     int    `json:"pid,omitempty"`
	Mode    string `json:"mode,omitempty"`    // off|normal|insert|visual|cmdline|raw|none ("none" or absent = no opinion); CLI set also: legacy|auto
	Focused *bool  `json:"focused,omitempty"` // nil = unknown
	Nested  bool   `json:"nested,omitempty"`  // nvim running inside another nvim's :terminal
	Tmux    bool   `json:"tmux,omitempty"`
	Plugin  string `json:"plugin,omitempty"`
	TTLMs   int    `json:"ttl_ms,omitempty"` // mode: the report expires after this, leaving the client with no opinion; set: override expiry
	Sticky  bool   `json:"sticky,omitempty"` // set: keep override across frontmost changes

	// Daemon → client / CLI.
	Daemon  string   `json:"daemon,omitempty"`
	Code    *uint8   `json:"code,omitempty"`
	Reason  string   `json:"reason,omitempty"`
	Err     string   `json:"err,omitempty"`
	Message string   `json:"msg,omitempty"`
	Status  *Status  `json:"status,omitempty"`
	Devices []Device `json:"devices,omitempty"`
}

// Status is the daemon's full state as returned to `zmk-vim-mode status`.
type Status struct {
	Version   string         `json:"version,omitempty"` // of the running daemon, not of the CLI asking
	Code      uint8          `json:"code"`
	Mode      string         `json:"mode"`
	Reason    string         `json:"reason"`
	Frontmost *FrontmostInfo `json:"frontmost,omitempty"`
	Clients   []ClientInfo   `json:"clients"`
	Override  *OverrideInfo  `json:"override,omitempty"`
	Devices   []Device       `json:"devices"`
	UptimeS   int64          `json:"uptime_s"`
}

// FrontmostInfo mirrors focus.App for status output.
type FrontmostInfo struct {
	Known bool   `json:"known"`
	Class string `json:"class,omitempty"`
	Title string `json:"title,omitempty"`
	PID   int    `json:"pid,omitempty"`
}

// ClientInfo describes one connected editor client.
type ClientInfo struct {
	ID        uint64 `json:"id"`
	Kind      string `json:"kind"`
	App       string `json:"app,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Mode      string `json:"mode"`
	Focused   *bool  `json:"focused,omitempty"`
	Nested    bool   `json:"nested,omitempty"`
	IdleMs    int64  `json:"idle_ms"`
	TTLLeftMs int64  `json:"ttl_left_ms,omitempty"`
}

// OverrideInfo describes an active manual override.
type OverrideInfo struct {
	Mode   string `json:"mode"`
	LeftMs int64  `json:"left_ms,omitempty"` // 0 = no TTL
	Sticky bool   `json:"sticky,omitempty"`
}

// Device describes one keyboard the daemon writes LED codes to.
type Device struct {
	ID        string `json:"id"`
	Product   string `json:"product,omitempty"`
	Transport string `json:"transport,omitempty"` // usb | bluetooth | unknown
	VID       uint16 `json:"vid,omitempty"`
	PID       uint16 `json:"pid,omitempty"`
	LastCode  *uint8 `json:"last_code,omitempty"`
	Writable  bool   `json:"writable"`
	Note      string `json:"note,omitempty"`
}

// Encode serialises m as one JSON line (with trailing newline).
func Encode(m Msg) ([]byte, error) {
	if m.T == "" {
		return nil, errors.New("proto: message type is required")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Decode parses one line (with or without trailing newline). Unknown fields are ignored.
func Decode(line []byte) (Msg, error) {
	line = bytes.TrimRight(line, "\r\n")
	if len(line) == 0 {
		return Msg{}, errors.New("proto: empty line")
	}
	if len(line) > MaxLine {
		return Msg{}, fmt.Errorf("proto: line too long (%d > %d)", len(line), MaxLine)
	}
	var m Msg
	if err := json.Unmarshal(line, &m); err != nil {
		return Msg{}, fmt.Errorf("proto: %w", err)
	}
	if m.T == "" {
		return Msg{}, errors.New("proto: missing t")
	}
	return m, nil
}

// Bool is a helper to build *bool fields.
func Bool(b bool) *bool { return &b }

// U8 is a helper to build *uint8 fields.
func U8(c uint8) *uint8 { return &c }
