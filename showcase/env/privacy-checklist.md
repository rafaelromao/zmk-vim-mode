# Privacy checklist — run before every take

Anything not on this list that shows a name, an address or an employer stops the take.

## System

- [ ] Waybar hidden on the recording monitor (it carries the clock, the network and any corporate
      tray icons); the HUD shows the layer instead
- [ ] Notifications silenced (`makoctl mode -s do-not-disturb`, or Omarchy's do-not-disturb toggle)
- [ ] Wallpaper: a plain dark colour on the recording monitor, no desktop widgets — an editor is
      maximized over it, but the moment before it is placed is on camera
- [ ] Wi-Fi / Bluetooth menus never opened on camera (network names)
- [ ] Teams, Outlook, Slack, browser profiles with your name: **quit**
- [ ] Karabiner has no counterpart here, but check no other tool is grabbing the Diamond: the HUD
      cannot read a device another process has seized (`bash showcase/hud.sh log` says so)

## Take hygiene (not privacy, but the first take lost clips to each of these)

- [ ] HUD drawing the board — not *waiting for the keymap…*
- [ ] Typed-keys strip empty; no `(rehearsal)` chip (the rehearsal HUD is not running)
- [ ] Mode line visible and showing the beat's expected reason
- [ ] No error line in any terminal on screen (pre-type the `journalctl … | grep` pipe before the take)
- [ ] Monitor scale set, waybar hidden, `hud.press_ms` raised for the take

## Terminal

- [ ] The demo terminal is Bash with `env/demo.bashrc` (see `env/ghostty-demo.conf`): prompt shows
      only the directory, and its history is `run/.bash_history`, not yours
- [ ] No `pwd`, `whoami`, `hostname`, `git log`, `env`, `history`, `atuin` on camera
- [ ] `zmk-vim-mode status` is fine to show: it prints window classes and pids, not names. Do **not**
      show `zmk-vim-mode doctor` output unedited — it prints `/home/<you>/…` paths; cover the wrap-up
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

- [ ] The HUD's typed-keys strip shows every key the *keyboard* sends (not the laptop's built-in
      keyboard, not injected keys): type no passwords, unlock codes or 2FA on the Diamond while recording
- [ ] The panel's corner title is the keyboard's own product name (*Diamond*, *Rommana*) unless
      `title:` in `~/.config/zmk-layer-hud/config.yaml` overrides it — check it says what you expect
- [ ] `zmk-vim-mode set` overrides cleared: `zmk-vim-mode status` shows no `override` line

## After recording

- [ ] Scrub the take at 2× looking at title bars, tab strips, status lines, the HUD banner (layer
      names only — the daemon's reason is not on screen) and the typed-keys strip
- [ ] Revert the global changes: IntelliJ zoom/font, waybar, do-not-disturb, any monitor scale you
      set for the take
