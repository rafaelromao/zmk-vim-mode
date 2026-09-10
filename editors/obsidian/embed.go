// Package obsidianplugin embeds the Obsidian plugin so the daemon binary can
// install it into a vault without a build step or a network.
package obsidianplugin

import "embed"

// Files are the plugin's two files: manifest.json and main.js.
//
//go:embed manifest.json main.js
var Files embed.FS
