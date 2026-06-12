package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	gohtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// faviconSVG is the mdserve mark, served at /favicon.svg and /favicon.ico so a
// browser's default favicon probe gets a 200 instead of a 404. (The in-page
// favicon is a theme-colored data-URI managed by the client.)
const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><rect width="32" height="32" rx="7" fill="#2f81f7"/><path d="M9 11h14M9 16h14M9 21h9" fill="none" stroke="#fff" stroke-width="2.4" stroke-linecap="round"/></svg>`

// Options configures a Server.
type Options struct {
	Dir        string // directory of .md files
	DefaultDoc string // rel path opened first when none is in the URL hash / history
	Reload     bool   // when true (serve), the page polls /api/poll for live-reload
}

// Server serves the single-page Markdown reader and its data endpoints. Markdown
// is rendered in the browser (marked.js) from /raw; the file tree comes from
// /api/tree; change detection from /api/poll; vendor assets from /vendor/. No
// file is cached server-side — each request re-reads from disk.
type Server struct {
	docDir string
	opts   Options
	page   string // pageHTML with the per-server sentinels substituted
}

// NewServer constructs a Server rooted at opts.Dir.
func NewServer(opts Options) (*Server, error) {
	if opts.DefaultDoc == "" {
		opts.DefaultDoc = "README.md"
	}
	abs, err := filepath.Abs(opts.Dir)
	if err != nil {
		return nil, fmt.Errorf("abs(%s): %w", opts.Dir, err)
	}
	if info, err := os.Stat(abs); err != nil {
		return nil, fmt.Errorf("stat %s: %w", abs, err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", abs)
	}
	reload := "false"
	if opts.Reload {
		reload = "true"
	}
	page := strings.NewReplacer(
		"__RELOAD__", reload,
		"__DEFAULT__", opts.DefaultDoc,
		"__VERSION__", versionString(),
	).Replace(pageHTML)
	return &Server{docDir: abs, opts: opts, page: page}, nil
}

// ServeHTTP routes:
//
//	GET /                 → the reader shell (HTML)
//	GET /api/tree         → JSON folder tree {root, rootPath, tree:[...]}
//	GET /api/poll         → JSON {relpath: mtime} for live-reload
//	GET /raw?path=<rel>   → raw Markdown bytes (path-safe, .md only)
//	GET /vendor/<name>    → embedded vendor asset
//	anything else         → 404
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch p := r.URL.Path; {
	case p == "/" || p == "/index.html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, s.page)
	case p == "/favicon.svg" || p == "/favicon.ico":
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = io.WriteString(w, faviconSVG)
	case p == "/api/tree":
		writeJSON(w, map[string]any{
			"root":     filepath.Base(s.docDir),
			"rootPath": s.docDir,
			"tree":     buildAPITree(s.docDir, ""),
		})
	case p == "/api/poll":
		writeJSON(w, s.flattenMtimes())
	case p == "/raw":
		s.serveRaw(w, r)
	case strings.HasPrefix(p, "/vendor/"):
		serveVendor(w, r, strings.TrimPrefix(p, "/vendor/"))
	default:
		http.NotFound(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// serveRaw returns the raw Markdown for ?path=<rel>, resolved safely inside
// docDir.
func (s *Server) serveRaw(w http.ResponseWriter, r *http.Request) {
	full, ok := s.resolveMd(r.URL.Query().Get("path"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	b, err := os.ReadFile(full)
	if err != nil {
		http.Error(w, "read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

// resolveMd maps a slash rel path to an absolute .md file inside docDir, or
// returns ok=false. path.Clean on a rooted copy neutralizes "../" traversal,
// and a prefix check guards against any residual escape.
func (s *Server) resolveMd(rel string) (string, bool) {
	clean := filepath.FromSlash(path.Clean("/" + rel))
	full := filepath.Join(s.docDir, clean)
	sep := string(filepath.Separator)
	if full != s.docDir && !strings.HasPrefix(full+sep, s.docDir+sep) {
		return "", false
	}
	if !strings.HasSuffix(strings.ToLower(full), ".md") {
		return "", false
	}
	if info, err := os.Stat(full); err != nil || info.IsDir() {
		return "", false
	}
	return full, true
}

// apiNode is one entry in the /api/tree JSON: a directory (with Children) or a
// .md leaf (with Mtime). Shapes match the reader's client expectations.
type apiNode struct {
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	Relpath  string    `json:"relpath"`
	Mtime    float64   `json:"mtime,omitempty"`
	Children []apiNode `json:"children,omitempty"`
}

// buildAPITree builds a recursive .md tree under absDir: directories first
// (pruned when they contain no markdown), then files, each group sorted
// case-insensitively. Hidden dirs and the build output (_site) are skipped.
func buildAPITree(absDir, rel string) []apiNode {
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return []apiNode{}
	}
	var dirs, files []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			if !strings.HasPrefix(e.Name(), ".") && e.Name() != "_site" {
				dirs = append(dirs, e)
			}
		} else if strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			files = append(files, e)
		}
	}
	byNameFold := func(s []os.DirEntry) {
		sort.Slice(s, func(i, j int) bool {
			return strings.ToLower(s[i].Name()) < strings.ToLower(s[j].Name())
		})
	}
	byNameFold(dirs)
	byNameFold(files)

	out := []apiNode{}
	for _, d := range dirs {
		childRel := relJoin(rel, d.Name())
		if ch := buildAPITree(filepath.Join(absDir, d.Name()), childRel); len(ch) > 0 {
			out = append(out, apiNode{Type: "dir", Name: d.Name(), Relpath: childRel, Children: ch})
		}
	}
	for _, f := range files {
		var mt float64
		if info, err := f.Info(); err == nil {
			mt = float64(info.ModTime().UnixNano()) / 1e9
		}
		out = append(out, apiNode{Type: "file", Name: f.Name(), Relpath: relJoin(rel, f.Name()), Mtime: mt})
	}
	return out
}

// stats counts the .md files served and the distinct directories that contain
// them (including the root) — reported at startup.
func (s *Server) stats() (files, dirs int) {
	m := s.flattenMtimes()
	files = len(m)
	seen := map[string]struct{}{".": {}}
	for rel := range m {
		for d := path.Dir(rel); d != "." && d != "/" && d != ""; d = path.Dir(d) {
			seen[d] = struct{}{}
		}
	}
	return files, len(seen)
}

func relJoin(rel, name string) string {
	if rel == "" {
		return name
	}
	return rel + "/" + name
}

// renderMarkdown converts a Markdown buffer to HTML (tables, fenced code,
// autolinks, footnotes, heading IDs). Used by the static build only — the live
// server renders Markdown in the browser.
func renderMarkdown(src []byte) []byte {
	exts := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(exts)
	renderer := gohtml.NewRenderer(gohtml.RendererOptions{Flags: gohtml.CommonFlags | gohtml.HrefTargetBlank})
	return markdown.Render(p.Parse(src), renderer)
}

// extractTitle returns the first H1, or "" if the file doesn't lead with one.
func extractTitle(src []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(src)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

var htmlEsc = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// BuildStatic renders each .md under docDir to a standalone HTML page under
// outDir, and copies the embedded vendor bundle alongside, so the result is a
// fully-offline static site. Used by `mdserve build`.
func (s *Server) BuildStatic(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := copyVendor(outDir); err != nil {
		return err
	}
	entries := s.flattenMtimes()
	rels := make([]string, 0, len(entries))
	for rel := range entries {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		raw, err := os.ReadFile(filepath.Join(s.docDir, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		title := extractTitle(raw)
		if title == "" {
			title = path.Base(rel)
		}
		depth := strings.Count(rel, "/")
		root := strings.Repeat("../", depth)
		body := string(renderMarkdown(raw))
		shell := strings.NewReplacer("__ROOT__", root, "__TITLE__", htmlEsc.Replace(title)).Replace(buildShell)
		shell = strings.Replace(shell, "__BODY__", body, 1)
		outPath := filepath.Join(outDir, filepath.FromSlash(rel)+".html")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(outPath, []byte(shell), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// statusWriter records the response status code for request logging.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// useColor enables ANSI coloring when stdout is a terminal and NO_COLOR is unset.
var useColor = func() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}()

// paint wraps s in an ANSI SGR code (no-op when color is disabled).
func paint(code, s string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func statusColor(c int) string {
	switch {
	case c >= 500:
		return "31" // red
	case c >= 400:
		return "33" // yellow
	case c >= 300:
		return "36" // cyan
	default:
		return "32" // green
	}
}

func durColor(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return "90" // gray (fast)
	case d < 25*time.Millisecond:
		return "32" // green
	case d < 150*time.Millisecond:
		return "33" // yellow
	default:
		return "31" // red
	}
}

// logRequests prints one aligned, colorized line per request — like a structured
// access logger:
//
//	15:04:05.000  200  GET   1.204ms  /raw?path=README.md
//
// Columns: time (gray), status (by class), method (gray), duration (by speed),
// path. The 2s poll and the static vendor assets are skipped so the log stays a
// readable record of page / doc / API views.
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/poll" || strings.HasPrefix(r.URL.Path, "/vendor/") {
			h.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		h.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		dur := time.Since(start).Round(time.Microsecond)
		target := r.URL.Path
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		fmt.Printf("%s  %s  %s  %s  %s\n",
			paint("90", start.Format("15:04:05.000")),
			paint(statusColor(sw.status), fmt.Sprintf("%3d", sw.status)),
			paint("90", fmt.Sprintf("%-4s", r.Method)),
			paint(durColor(dur), fmt.Sprintf("%10s", dur.String())),
			target,
		)
	})
}
