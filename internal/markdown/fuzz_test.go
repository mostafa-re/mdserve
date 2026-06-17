package markdown

import (
	"strings"
	"testing"
)

// FuzzRender checks the renderer never panics and never leaks internal sentinel
// bytes (\x00 atom placeholders, \x01 hard-break marker) into its output.
func FuzzRender(f *testing.F) {
	for _, s := range []string{
		"# h", "*a*", "**a**", "***a***", "_a_b_", "~~x~~", "`c`",
		"[t](u)", "![a](u)", "<http://x>", "www.x.com", "- a\n- b",
		"1. a\n  2. b", "> q", "| a |\n|---|\n| 1 |", "```go\nx\n```",
		"a\x00b", "a\x01b", "## ##", "# T {#a\"x}", "$x$",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		out := string(Render([]byte(s)))
		if strings.ContainsAny(out, "\x00\x01") {
			t.Fatalf("output leaked a sentinel byte for input %q", s)
		}
	})
}
