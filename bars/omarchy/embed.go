// Package omarchybar embeds the Omarchy Quattro bar widget, so the daemon
// binary can install it without a build step or a network.
package omarchybar

import "embed"

// Files holds the plugin's directory, named after its id as Omarchy requires:
// rafaelromao.zmk-vim-mode.
//
//go:embed rafaelromao.zmk-vim-mode
var Files embed.FS
