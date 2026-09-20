# YouTube packaging

Companion to `SCRIPT.md`: everything around the video itself.

## Title

Pick one; the two others are the A/B candidates for the first 24 hours.

1. *My 24-key keyboard knows which vim mode I'm in*
2. *Vim on an alternative layout, without remapping vim*
3. *The keyboard that knows your vim mode (ZMK + Neovim)*

## Thumbnail

`Diamond.jpeg` from the keyboards repo on the left, the HUD board (a frame from the recording)
large on the right, two words across the top: `NORMAL → INSERT`. High contrast on the dark
theme, at most four words, no logos.

## Description

```
The keyboard switches layers when vim changes mode. No remapping in vim: the editor tells the
keyboard its exact mode over three LED indicator bits no OS ever sets, and a ZMK module flips
the layer. Works in Neovim, VS Code (vscode-neovim), IntelliJ (IdeaVim) and Obsidian.

0:00 The keyboard follows Escape
0:36 What this video is about
1:18 Why alternative layouts break vim
2:09 How it works: three LED bits
3:12 Neovim, including raw mode
4:23 VS Code: four ways to know
5:31 IntelliJ IDEA
6:19 Obsidian
6:57 Everywhere else and the escape hatch
7:44 Install

zmk-vim-mode   https://github.com/rafaelromao/zmk-vim-mode
zmk-layer-hud  https://github.com/rafaelromao/zmk-layer-hud   (the live keymap on the right; any ZMK board)
keymap         https://github.com/rafaelromao/keyboards
Romak          https://rafaelromao.github.io/romak

Hardware: the Diamond, 24 keys, ZMK. Layout: Magic Romak.
```

Fill the chapter times from the final cut; the ones above are the script's.

## Tags

zmk, ergonomic keyboard, split keyboard, mechanical keyboard, neovim, vim, colemak,
alternative layout, keymap, HID, vscode-neovim, ideavim, obsidian, keyboard layers

## Pinned comment

The "how it works" paragraph from `SCRIPT.md` (the LED-bit channel, in five lines), the four
links, and one question: *which layout do you type on?*

## Where to post, in this order

1. ZMK Discord, *#showcase* — lead with the HUD and the module; that audience will ask for
   zmk-layer-hud on its own.
2. r/ErgoMechKeyboards — lead with the hardware: 24 keys, combos, the board following the editor.
3. r/neovim — lead with the plugin and *raw*: dashboards, pickers, leader, terminal.
4. r/vim, then Hacker News as *Show HN* with the LED-bit trick in the title.
5. keymap-drawer's discussions, for the HUD (it draws the drawer's own YAML live).

## The Short

The 18-second cold open re-framed as a 9:16 vertical from the same screen recording: the HUD
on top, the editor's cursor line below it. Post the same day as the
long video; it feeds it.

## Second video candidate

zmk-layer-hud alone — a live keymap-drawer HUD from the keyboard's own reports, for any ZMK
board, with or without vim. The ergo audience will ask for it in the comments of this one.
