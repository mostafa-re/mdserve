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
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
)

// pseudoVersion matches Go's synthetic module pseudo-versions, whose tail is a
// 14-digit UTC timestamp and a 12-hex commit (e.g. v0.3.1-0.20260612135040-deaa63af60d0).
var pseudoVersion = regexp.MustCompile(`[0-9]{14}-[0-9a-f]{12}`)

const usage = `mdserve — serve a directory of Markdown as browsable HTML.

usage:
  mdserve serve [flags]     start a live server (default if no command given)
  mdserve build [flags]     render to a static HTML tree and exit
  mdserve update [--check]  replace this binary with the latest release
  mdserve version           print the version (release tag, or dev+<commit>)

serve flags:
  --dir string          directory of .md files (default ".", the current dir)
  --addr string         listen address; falls back to a free port if taken (default "127.0.0.1:8080")
  --default-doc string  doc opened first (default "README.md")
  --exclude glob        hide dirs/files matching a glob (repeatable, comma-separated)
  --open                open the default browser at the served URL
  --no-reload           disable live-reload polling
  --no-update-check     don't check GitHub for a newer release on launch
  --no-cdn              deprecated: vendor assets are always embedded (no-op)

build flags:
  --dir string          directory of .md files (default ".", the current dir)
  --out string          output directory (required)
  --default-doc string  index doc (default "README.md")
  --exclude glob        skip dirs/files matching a glob (repeatable, comma-separated)
  --no-cdn              deprecated: vendor assets are always embedded (no-op)

The update check (serve) skips dev builds, caches its result ~24h, fails
silently, and can be disabled with --no-update-check or MDSERVE_NO_UPDATE_CHECK.
`

// version is the release tag, injected via -ldflags -X main.version=<tag> by the
// release workflow. Local builds leave it "dev"; versionString then derives a
// dev+<commit> string from Go's embedded VCS stamp.
var version = "dev"

// repoSlug is the GitHub owner/repo used by `mdserve update` and install.sh.
const repoSlug = "mostafa-re/mdserve"

// versionString reports the build version: the release tag baked in via -ldflags,
// else the module version from `go install <pkg>@vX`, else "dev+<short-commit>"
// from the embedded VCS stamp (with -dirty when the tree was modified).
func versionString() string {
	if version != "dev" && version != "" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	// A clean release tag from `go install <pkg>@vX.Y.Z`. Ignore Go's synthetic
	// pseudo-versions (vX.Y.Z-0.<timestamp>-<commit>) — for those the user wants
	// the dev+<commit> form below, not the noisy pseudo-version.
	if v := info.Main.Version; v != "" && v != "(devel)" && !pseudoVersion.MatchString(v) {
		return v
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	v := "dev+" + rev
	if modified == "true" {
		v += "-dirty"
	}
	return v
}

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
	case "update", "self-update", "upgrade":
		runUpdate(args)
	case "version", "--version", "-v":
		line := "mdserve " + versionString()
		if ind := updateIndicator(os.Getenv("MDSERVE_NO_UPDATE_CHECK") == ""); ind != "" {
			line += "   " + ind
		}
		fmt.Println(line)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintln(os.Stderr, "mdserve: unknown command:", cmd)
		fmt.Print(usage)
		os.Exit(2)
	}
}

// stringList is a flag.Value collecting --exclude: repeatable and
// comma-separated, so --exclude a --exclude b,c yields [a b c].
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error {
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			*s = append(*s, p)
		}
	}
	return nil
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dir := fs.String("dir", defaultDir, "directory of .md files")
	addr := fs.String("addr", defaultAddr, "listen address (free-port fallback if taken)")
	defDoc := fs.String("default-doc", "README.md", "doc opened at /")
	var exclude stringList
	fs.Var(&exclude, "exclude", "glob(s) to hide from serving (repeatable, comma-separated)")
	open := fs.Bool("open", false, "open the browser")
	noReload := fs.Bool("no-reload", false, "disable live-reload polling")
	noUpdate := fs.Bool("no-update-check", false, "don't check GitHub for a newer release on launch")
	noCDN := fs.Bool("no-cdn", false, "deprecated: vendor assets are always embedded (no-op)")
	_ = fs.Parse(args)
	_ = noCDN

	updateCheck := !*noUpdate && os.Getenv("MDSERVE_NO_UPDATE_CHECK") == ""
	srv, err := NewServer(Options{Dir: *dir, DefaultDoc: *defDoc, Reload: !*noReload, Exclude: exclude})
	if err != nil {
		fatal(err)
	}
	ln, shown, err := listen(*addr)
	if err != nil {
		fatal(err)
	}
	url := "http://" + shown + "/"
	files, dirs := srv.stats()
	fmt.Println(banner)
	fmt.Printf("  %s\n", versionString())
	fmt.Printf("  %s\n", srv.docDir)
	fmt.Printf("  %s · %s\n\n", pl(files, "markdown file", "markdown files"), pl(dirs, "directory", "directories"))
	fmt.Printf("  ➜  %s\n", url)
	fmt.Printf("  %s\n\n", paint("90", "Ctrl+C to stop"))
	// URL is already shown; the release check (cached ~24h, ≤5s) prints below it.
	if ind := updateIndicator(updateCheck); ind != "" {
		fmt.Printf("  %s\n\n", ind)
	}
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
	var exclude stringList
	fs.Var(&exclude, "exclude", "glob(s) to skip from the build (repeatable, comma-separated)")
	noCDN := fs.Bool("no-cdn", false, "deprecated: vendor assets are always embedded (no-op)")
	_ = fs.Parse(args)
	_ = noCDN
	if *out == "" {
		fatal(fmt.Errorf("build requires --out"))
	}
	srv, err := NewServer(Options{Dir: *dir, DefaultDoc: *defDoc, Exclude: exclude})
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
		// the requested port was busy — fall back to a free one, quietly.
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

// pl renders a count with a singular/plural noun: pl(1,"file","files") → "1 file".
func pl(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}
