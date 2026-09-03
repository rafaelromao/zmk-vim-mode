# zmk-vim-mode

Keeps a ZMK keyboard's layers in sync with the editor's vim state (Neovim first; VSCode via
vscode-neovim, Obsidian via a plugin), on Linux (Omarchy/Hyprland) and later macOS.

Host daemon (Go) + Neovim plugin (Lua) + ZMK firmware module (C) in one repo. The signal channel is the
HID LED indicator output report (Compose / Kana / Scroll Lock bits = a 3-bit mode code).

See [PLAN.md](PLAN.md) for the full design, decisions, phases and verification plan.

Status: planning complete, implementation not started.
