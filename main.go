// Command mdserve serves a directory of Markdown files as browsable HTML, or
// renders them to a static site: free-port fallback, browser auto-open, and
// live-reload on save. Intentionally dependency-light — one markdown library,
// stdlib for everything else.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const usage = `mdserve — serve a directory of Markdown as browsable HTML.

usage:
  mdserve serve [flags]     start a live server (default if no command given)
  mdserve build [flags]     render to a static HTML tree and exit
  mdserve version

serve flags:
  --dir string          directory of .md files (default ".", the current dir)
  --addr string         listen address; falls back to a free port if taken (default "127.0.0.1:8080")
  --default-doc string  doc opened at / (default "README.md")
  --open                open the default browser at the served URL
  --no-cdn              don't reference CDN assets (mermaid / highlight.js)
  --no-reload           disable live-reload-on-save

build flags:
  --dir string          directory of .md files (default ".", the current dir)
  --out string          output directory (required)
  --default-doc string  index doc linked at / (default "README.md")
  --no-cdn              don't reference CDN assets
`

// version is overridable via -ldflags -X main.version=...
var version = "dev"

// defaultAddr binds loopback, not the wildcard ":8080". A wildcard bind succeeds
// even when another process already holds 127.0.0.1:8080, which would shadow the
// advertised http://127.0.0.1:8080/ URL with that other process's responses; a
// loopback bind instead fails with EADDRINUSE so listen() falls back to a free
// port. Loopback also keeps a local docs server off the LAN.
const defaultAddr = "127.0.0.1:8080"

// defaultDir is the current working directory: a bare `mdserve` (no flags)
// serves whatever repo you're standing in, instead of assuming a ./docs subdir.
const defaultDir = "."

// banner is printed once at serve start.
const banner = `
  ┌────────────────────────────────────────┐
  │   mdserve · browse markdown as html    │
  └────────────────────────────────────────┘`

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	switch cmd {
	case "serve":
		runServe(args)
	case "build":
		runBuild(args)
	case "version":
		fmt.Println("mdserve", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintln(os.Stderr, "mdserve: unknown command:", cmd)
		fmt.Print(usage)
		os.Exit(2)
	}
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dir := fs.String("dir", defaultDir, "directory of .md files")
	addr := fs.String("addr", defaultAddr, "listen address (free-port fallback if taken)")
	defDoc := fs.String("default-doc", "README.md", "doc opened at /")
	open := fs.Bool("open", false, "open the browser")
	noCDN := fs.Bool("no-cdn", false, "no CDN assets")
	noReload := fs.Bool("no-reload", false, "disable live-reload")
	_ = fs.Parse(args)

	srv, err := NewServer(Options{Dir: *dir, DefaultDoc: *defDoc, NoCDN: *noCDN, LiveReload: !*noReload})
	if err != nil {
		fatal(err)
	}
	ln, shown, err := listen(*addr)
	if err != nil {
		fatal(err)
	}
	url := "http://" + shown + "/"
	if srv.opts.LiveReload {
		srv.startWatch()
	}
	fmt.Println(banner)
	fmt.Printf("  %s\n\n", version)
	fmt.Printf("mdserve: serving %s on %s\n", *dir, url)
	if *open {
		openBrowser(url)
	}
	httpSrv := &http.Server{Handler: logRequests(srv)} //nolint:gosec // local dev server
	if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func runBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	dir := fs.String("dir", defaultDir, "directory of .md files")
	out := fs.String("out", "", "output directory (required)")
	defDoc := fs.String("default-doc", "README.md", "index doc")
	noCDN := fs.Bool("no-cdn", false, "no CDN assets")
	_ = fs.Parse(args)
	if *out == "" {
		fatal(fmt.Errorf("build requires --out"))
	}
	srv, err := NewServer(Options{Dir: *dir, DefaultDoc: *defDoc, NoCDN: *noCDN})
	if err != nil {
		fatal(err)
	}
	if err := srv.BuildStatic(*out); err != nil {
		fatal(err)
	}
	fmt.Printf("mdserve: rendered %s -> %s\n", *dir, *out)
}

// listen binds addr; if its port is already in use, it retries on an
// OS-assigned free port (:0), preserving the host part. net.Listen on :0 is the
// race-free way to grab a guaranteed-free port — never scan a range.
func listen(addr string) (net.Listener, string, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		host, _, splitErr := net.SplitHostPort(addr)
		if splitErr != nil {
			host = ""
		}
		ln, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			return nil, "", err
		}
		fmt.Fprintf(os.Stderr, "mdserve: %s busy — using a free port\n", addr)
	}
	return ln, displayAddr(ln), nil
}

// displayAddr renders a listener's address with an unspecified host (0.0.0.0/::)
// shown as 127.0.0.1 so the printed URL is clickable.
func displayAddr(ln net.Listener) string {
	a := ln.Addr().(*net.TCPAddr)
	host := a.IP.String()
	if a.IP.IsUnspecified() {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, strconv.Itoa(a.Port))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "mdserve:", err)
	os.Exit(1)
}
