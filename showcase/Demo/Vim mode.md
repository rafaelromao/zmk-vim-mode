# Vim mode

The host encodes the editor's state as a 3-bit number in the HID LED report,
on the three indicators no operating system drives: Compose, Kana and Scroll Lock.

| code | state   | keyboard layers        |
|-----:|---------|------------------------|
| 0    | off     | none                   |
| 1    | normal  | VIM_NORMAL             |
| 2    | insert  | VIM_INSERT             |
| 3    | visual  | VIM_NORMAL, VIM_VISUAL |
| 4    | legacy  | VIM_NORMAL (inferred)  |
| 5    | cmdline | VIM_CMDLINE            |
| 6    | raw     | none                   |
| 7    | legacy  | VIM_NORMAL, silent     |

The firmware tests three bits, rebuilds the integer, and if it changed switches
the layer set. That is a bitmask change, so it is effectively instant.

## Raw

Reading view, the sidebar, search and the note title report **raw**: the keys
must reach Obsidian untouched, so the keyboard shows its plain alpha layout there.
