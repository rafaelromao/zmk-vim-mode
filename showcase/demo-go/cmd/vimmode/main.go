// Command vimmode prints the code table, or decodes an LED byte given in hex.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"example.com/vimmode/internal/modes"
)

func main() {
	if len(os.Args) > 1 {
		v, err := strconv.ParseUint(strings.TrimPrefix(os.Args[1], "0x"), 16, 8)
		if err != nil {
			fmt.Fprintln(os.Stderr, "usage: vimmode [ledbyte-hex]")
			os.Exit(2)
		}
		m := modes.Decode(byte(v))
		fmt.Printf("%#02x → %d (%s) %v\n", v, m, m, m.Layers())
		return
	}
	fmt.Println("code  byte  mode           layers")
	for m := modes.Off; m <= modes.LegacySilent; m++ {
		fmt.Printf("%-5d %#02x  %-14s %s\n", m, modes.Encode(m, 0), m, strings.Join(m.Layers(), " "))
	}
}
