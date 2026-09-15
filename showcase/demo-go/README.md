# vimmode

A tiny Go module used in the zmk-vim-mode showcase: the eight editor states, how
they are packed into the HID LED indicator byte, and which keyboard layers each
one activates.

```
go run ./cmd/vimmode        # the table
go run ./cmd/vimmode 0x18   # decode one byte
go test ./...
```
