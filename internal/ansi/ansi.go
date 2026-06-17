// Package ansi is a tiny terminal-coloring helper shared by the CLI and the
// server's request logger. It honors NO_COLOR and only colors a real TTY.
package ansi

import "os"

// useColor enables ANSI coloring when stdout is a terminal and NO_COLOR is unset.
var useColor = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

// Paint wraps s in an ANSI SGR code (no-op when color is disabled).
func Paint(code, s string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}
