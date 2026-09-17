# The README animation

`../img/vim-layers.gif` shows the keyboard's layers following the editor's vim state, drawn by
[zmk-layer-hud](https://github.com/rafaelromao/zmk-layer-hud) from the author's Diamond keymap.

It is a **scripted rendering of the HUD page**, not a screen capture: the demo script
(`docs/demo-vim.json` in zmk-layer-hud, beside the Diamond config it is written against) names,
for each frame, the ZMK layer ids the keyboard reports, the key positions that are down, and the
chips on the typed-keys strip. The HUD draws each frame through the same calls the live host
makes (`hud.setLayers`, `hud.pressAt`), so a frame shows what the HUD would show — a combo
resolves to its pill on the topmost active layer exactly as a real chord does.

Layer ids are the Diamond's, from `config.dtsi` (`VIM_NORMAL` 2, `VIM_VISUAL` 3, `VIM_INSERT` 5).
`VIM_CMDLINE` is not in the demo, and `VIM_CHANGE` and `VIM_REPLACE` are left out on purpose: the
keyboard manages them, but no code from the daemon ever activates them
(see [../keyboards-repo.md](../keyboards-repo.md)).

The transitions are the keymap's own, so the animation reads as a real session: `v` (the combo on
`i` and `a`) enters VISUAL, `i` or `a` enters INSERT, and Esc leaves either — `b` + `e` on the vim
layer, the same two positions as `b` + `m` on Alpha 1 while INSERT is up. Key positions come from
the `positions:` list in zmk-layer-hud's `config/diamond.yaml` (`h` 16, `j` 17, `k` 18, `l` 19,
`w` 3, `i` 26, `a` 27).

## Regenerating it

Needs zmk-layer-hud checked out beside this repo, its venv (`make venv`), `ffmpeg`, and a
Chromium-family browser. Run it from a normal shell — a sandbox that denies unix sockets or TCP
binds cannot start either the browser or the page's http server.

```bash
cd ~/projects/zmk-layer-hud && bash docs/make-gif.sh --config config/diamond.yaml --script docs/demo-vim.json --out ~/projects/zmk-vim-mode/docs/img/vim-layers.gif --framerate 0.9
```

The keymap itself is read live from `~/projects/keyboards`, so the legends always match the
published diagrams. The script's shape is documented above `demoFrame` in zmk-layer-hud's
`hud/hud.js`. The same animation is that repository's own README image, so a change here is
worth rendering to `docs/hud.gif` there as well.
