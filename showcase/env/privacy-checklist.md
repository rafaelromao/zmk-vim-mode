# Privacy checklist — run before every take

Anything not on this list that shows a name, an address or an employer stops the take.

## System

- [ ] System Settings → Desktop & Dock → *Automatically hide and show the menu bar*: **Always**
      (hides the corporate agents, the clock, and the Hammerspoon VIM indicator; the HUD replaces it)
- [ ] Focus → Do Not Disturb on (no notification banners)
- [ ] Wallpaper: a plain dark colour (System Settings → Wallpaper → Colours) on the recording display;
      Desktop icons off or the Desktop empty — the editors are placed above the typed-keys band, so a
      strip of desktop shows beneath them
- [ ] Dock hidden (`⌥⌘D`) — it shows app badges and running corporate tools
- [ ] Bluetooth/Wi-Fi menus never opened on camera (network names)
- [ ] Teams, Outlook, Slack, browser profiles with your name: **quit**
- [ ] AirDrop / Sharing panes never opened (hostname)

## Terminal

- [ ] Ghostty started with `ZDOTDIR=showcase/env` and `ghostty-demo.conf`: prompt shows only the directory
- [ ] No `pwd`, `whoami`, `hostname`, `git log`, `env`, `history`, `atuin` on camera
- [ ] `zmk-vim-mode status` is fine to show: it prints bundle ids and pids, not names. Do **not**
      show `zmk-vim-mode doctor` output unedited — it prints `/Users/<you>/…` paths; cover the wrap-up
      with a pre-rendered terminal or blur the path column
- [ ] Neovim: dashboard, `:Lazy`, `:Mason` are fine; never open `~/.config`, `:messages`, `:checkhealth`

## Editors

- [ ] VS Code: only `showcase/demo-go` open; title bar shows `modes.go — demo-go [Text Editor]`,
      no account avatar menu opened; Settings UI never opened (shows sync account)
- [ ] IntelliJ: only `demo-java`; no *Help → About*, no *Settings → Account*; the project path shows in
      the title bar only if *Appearance → Show full paths* is on — keep it off
- [ ] Obsidian: only the `Demo` vault (showcase/Demo); the vault switcher lists your other vaults by name: never open it
- [ ] the demo projects are part of this repo: no `git log`/blame on camera (the commit author is your public identity anyway)

## Keyboard

- [ ] The typed-keys strip shows every keystroke: type no passwords, unlock codes or 2FA while recording
- [ ] `zmk-vim-mode set` overrides cleared: `zmk-vim-mode status` shows no `override` line

## After recording

- [ ] Scrub the take at 2× looking at title bars, tab strips, status lines and the HUD reason text
      (`non-editor app com.…` names apps by bundle id: acceptable; window titles are not shown)
- [ ] Revert the global changes: IntelliJ zoom/font, menu bar auto-hide, Dock, Focus mode
