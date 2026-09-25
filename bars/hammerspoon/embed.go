// Package hammerspoonbar embeds the Hammerspoon Spoon that shows the vim mode
// in the macOS menu bar, so the daemon binary can install it without a build
// step or a network.
package hammerspoonbar

import "embed"

// Files holds the Spoon's directory, ZmkVimMode.spoon.
//
//go:embed ZmkVimMode.spoon
var Files embed.FS
