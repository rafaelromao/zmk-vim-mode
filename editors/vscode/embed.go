// Package vscodeext embeds the VSCode companion so the daemon binary can
// package and install it without node, vsce or a network.
package vscodeext

import "embed"

// Files are the extension's sources: package.json, extension.js, README.md.
//
//go:embed package.json extension.js README.md
var Files embed.FS
