package main

import (
	"os"
	"os/exec"
	"runtime"
)

// openBrowser opens url in the default browser. Best-effort: a failure (headless,
// SSH, CI) is silently ignored — the URL is already printed to stdout.
func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	default: // linux, *bsd
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return // headless — nothing to open
		}
		cmd, args = "xdg-open", []string{url}
	}
	_ = exec.Command(cmd, args...).Start()
}
