# Menu bar indicator (Hammerspoon)

A [Hammerspoon](https://www.hammerspoon.org) Spoon that shows the vim mode your
keyboard is in: the Neovim glyph and NORMAL, INSERT, VISUAL, CMDLINE, VIM or
RAW, `VIM ?` while the daemon is down, and nothing at all while vim mode is
off. Once a second it runs `zmk-vim-mode status --bar`, the same answer the
Omarchy widget draws, and renders it; the wording lives in the daemon, not
here.

## Install

`make install` does it on macOS; `zmk-vim-mode install --hammerspoon` does it
alone. It:

- writes the Spoon to `~/.hammerspoon/Spoons/ZmkVimMode.spoon` (next to the
  `init.lua` named by Hammerspoon's `MJConfigFile` preference, if you moved
  it), with the path of the binary that installed it written in: Hammerspoon
  does not see your shell's `PATH`;
- appends these lines to `init.lua`, unless it already mentions `ZmkVimMode`:

  ```lua
  -- added by zmk-vim-mode: vim mode indicator in the menu bar
  if hs.fs.attributes(hs.configdir .. "/Spoons/ZmkVimMode.spoon/init.lua") then
    local ok, err = pcall(function() hs.loadSpoon("ZmkVimMode"):start() end)
    if not ok then print("ZmkVimMode: " .. tostring(err)) end
  end
  ```

  Guarded both ways: a removed Spoon is skipped and a broken one is reported
  in the console, so neither can stop the rest of your config from loading.
  Without an `init.lua` it creates one holding just these lines; an `init.lua`
  that is a symlink stays one, with the lines added to the file it points to;
- reloads Hammerspoon's config when Hammerspoon is running and has `hs.ipc`
  loaded (`require("hs.ipc")` in `init.lua`). Otherwise reload it yourself:
  menu bar icon → Reload Config.

`zmk-vim-mode doctor` checks the Spoon is there and that `init.lua` starts it.

## Settings

Set them before `:start()`. Lines of your own that mention `ZmkVimMode` replace
the installer's: a later install leaves them alone.

```lua
local vim = hs.loadSpoon("ZmkVimMode")
vim.interval = 1      -- seconds between looks at the daemon
vim.iconFont = nil    -- a Nerd Font family; nil takes the first one installed
vim.socket = nil      -- the daemon's socket; Hammerspoon does not see ZMK_VIM_MODE_SOCKET
vim:start()
```

The glyph (U+E6AE, `nf-custom-neovim`) needs a Nerd Font, because the menu
bar's own font has nothing in the Nerd Font private-use area. Any installed
one will do (`brew install --cask font-hack-nerd-font`, for instance); with
none, the item shows the mode alone.

## By hand

Copy `ZmkVimMode.spoon` into `~/.hammerspoon/Spoons/` and add
`hs.loadSpoon("ZmkVimMode"):start()` to `init.lua`. A copy that was not
installed looks for the binary in `~/.local/bin`, `/opt/homebrew/bin` and
`/usr/local/bin`.

For work on the Spoon itself, symlink the directory into `Spoons/` instead: the
installer leaves a symlinked Spoon alone rather than writing into your
checkout. `make test-lua` covers what it decides to show.
