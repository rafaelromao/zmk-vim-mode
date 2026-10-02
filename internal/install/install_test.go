package install

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

func TestSystemdArg(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/home/u/.local/bin/zmk-vim-mode", "/home/u/.local/bin/zmk-vim-mode"},
		// specifiers and variables expand anywhere on the line
		{"/opt/50%/$x/zmk-vim-mode", "/opt/50%%/$$x/zmk-vim-mode"},
		{"/home/u/my bin/zmk-vim-mode", `"/home/u/my bin/zmk-vim-mode"`},
		{`/opt/a"b\c/zmk-vim-mode`, `"/opt/a\"b\\c/zmk-vim-mode"`},
		{"/opt/a\nExecStartPre=/bin/true", `"/opt/a\nExecStartPre=/bin/true"`},
	}
	for _, c := range cases {
		if got := systemdArg(c.in); got != c.want {
			t.Errorf("systemdArg(%q) = %s, want %s", c.in, got, c.want)
		}
	}
	unit := fmt.Sprintf(systemdUnit, systemdArg("/opt/a\nExecStartPre=/bin/true"), "")
	for _, l := range strings.Split(unit, "\n") {
		if strings.HasPrefix(l, "ExecStartPre=") {
			t.Fatalf("a path broke out of ExecStart:\n%s", unit)
		}
	}
}

// The plist is XML: whatever the path, a parser must read back exactly it.
func TestLaunchdPlistEscapesPaths(t *testing.T) {
	exe := `/Users/u/a&b <c>/"q"/zmk-vim-mode`
	plist := fmt.Sprintf(launchdPlist, xmlText(exe), xmlText("/Users/u/Library/Logs/zmk-vim-mode.log"))
	dec := xml.NewDecoder(strings.NewReader(plist))
	dec.Strict = true
	var strs []string
	inString := false
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			t.Fatalf("plist does not parse: %v\n%s", err, plist)
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			inString = tk.Name.Local == "string"
		case xml.EndElement:
			inString = false
		case xml.CharData:
			if inString {
				strs = append(strs, string(tk))
			}
		}
	}
	found := false
	for _, s := range strs {
		if s == exe {
			found = true
		}
	}
	if !found {
		t.Fatalf("ProgramArguments lost the path; strings: %q", strs)
	}
}
