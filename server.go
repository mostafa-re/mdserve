package main

import (
	"bufio"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

// urlPrefix is the path under which docs are mounted (so a file rel path maps to
// a stable /docs/<rel> URL).
const urlPrefix = "/docs/"

// liveReloadPath is the SSE endpoint the browser subscribes to for reload pings.
const liveReloadPath = "/__mdserve_reload"

// Options configures a Server.
type Options struct {
	Dir        string // directory of .md files
	DefaultDoc string // rel path served at "/" (e.g. "README.md")
	NoCDN      bool   // when true, omit mermaid/highlight.js CDN assets
	LiveReload bool   // when true (serve only), inject the live-reload client
}

// Server walks the docs dir lazily and serves rendered Markdown over HTTP. Each
// request re-reads the file (no cache) so the page reflects the last save.
type Server struct {
	docDir string
	tmpl   *template.Template
	opts   Options

	mu      sync.Mutex
	clients map[chan struct{}]struct{} // live-reload subscribers
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
	tmpl, err := template.New("page").Parse(pageTmpl)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	return &Server{docDir: abs, tmpl: tmpl, opts: opts, clients: map[chan struct{}]struct{}{}}, nil
}

// ServeHTTP implements http.Handler:
//
//	GET /                    → 302 to /docs/<DefaultDoc>
//	GET /docs/<rel>.md       → render that file as HTML
//	GET /__mdserve_reload    → SSE stream (live-reload)
//	anything else            → 404
//
// Path traversal is blocked by keeping the resolved path within docDir.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/":
		http.Redirect(w, r, urlPrefix+s.opts.DefaultDoc, http.StatusFound)
		return
	case r.URL.Path == liveReloadPath:
		s.serveLiveReload(w, r)
		return
	case !strings.HasPrefix(r.URL.Path, urlPrefix):
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, urlPrefix)
	if !strings.HasSuffix(rel, ".md") {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(s.docDir, filepath.FromSlash(rel))
	if !strings.HasPrefix(full+string(filepath.Separator), s.docDir+string(filepath.Separator)) && full != s.docDir {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	title := extractTitle(raw)
	if title == "" {
		title = filepath.Base(rel)
	}
	nav, err := s.buildNav()
	if err != nil {
		http.Error(w, "nav: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := pageData{
		Title:      title,
		Body:       template.HTML(renderMarkdown(raw)), //nolint:gosec // local docs
		Nav:        nav,
		Active:     rel,
		LiveReload: s.opts.LiveReload,
		CDN:        !s.opts.NoCDN,
	}
	if err := s.tmpl.Execute(w, data); err != nil {
		fmt.Fprintln(os.Stderr, "mdserve: template execute:", err)
	}
}

// renderMarkdown converts a Markdown buffer to safe HTML (tables, fenced code,
// autolinks, footnotes, heading IDs).
func renderMarkdown(src []byte) []byte {
	exts := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(exts)
	renderer := html.NewRenderer(html.RendererOptions{Flags: html.CommonFlags | html.HrefTargetBlank})
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

type navEntry struct {
	URL  string
	Name string
}

func (s *Server) buildNav() ([]navEntry, error) {
	var entries []navEntry
	err := filepath.WalkDir(s.docDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "_site" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(s.docDir, p)
		if err != nil {
			return err
		}
		entries = append(entries, navEntry{URL: urlPrefix + filepath.ToSlash(rel), Name: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
	return entries, nil
}

type pageData struct {
	Title      string
	Body       template.HTML
	Nav        []navEntry
	Active     string
	LiveReload bool
	CDN        bool
}

// BuildStatic walks docDir and writes a fully-rendered HTML tree under outDir
// (no live-reload script). Used by `mdserve build`.
func (s *Server) BuildStatic(outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	nav, err := s.buildNav()
	if err != nil {
		return err
	}
	return filepath.WalkDir(s.docDir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, err := filepath.Rel(s.docDir, p)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		title := extractTitle(raw)
		if title == "" {
			title = path.Base(rel)
		}
		outPath := filepath.Join(outDir, rel+".html")
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return err
		}
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		data := pageData{
			Title:  title,
			Body:   template.HTML(renderMarkdown(raw)), //nolint:gosec // local docs
			Nav:    nav,
			Active: filepath.ToSlash(rel),
			CDN:    !s.opts.NoCDN,
		}
		return s.tmpl.Execute(f, data)
	})
}
