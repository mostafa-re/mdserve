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
	"time"

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

// tmplFuncs are the template helpers the page shell needs: hasPrefix (open the
// dir on the active doc's path) and dict (pass multiple values into the
// recursive "tree" sub-template).
var tmplFuncs = template.FuncMap{
	"hasPrefix": strings.HasPrefix,
	"dict": func(pairs ...any) map[string]any {
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i+1 < len(pairs); i += 2 {
			key, _ := pairs[i].(string)
			m[key] = pairs[i+1]
		}
		return m
	},
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
	tmpl, err := template.New("page").Funcs(tmplFuncs).Parse(pageTmpl)
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
	tree, err := s.buildTree()
	if err != nil {
		http.Error(w, "nav: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := pageData{
		Title:      title,
		Body:       template.HTML(renderMarkdown(raw)), //nolint:gosec // local docs
		Tree:       tree,
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

// treeNode is one entry in the nav folder tree: a directory (with Children) or a
// leaf .md file (with URL). Rel is the slash path from docDir — for a dir it is
// the dir path, used to auto-open the branch leading to the active doc.
type treeNode struct {
	Name     string
	Rel      string
	URL      string
	IsDir    bool
	Children []*treeNode
}

// buildTree walks the docs dir and assembles a nested folder tree of .md files,
// directories first then files, alphabetical within each level.
func (s *Server) buildTree() ([]*treeNode, error) {
	root := &treeNode{IsDir: true}
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
		insertPath(root, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortTree(root)
	return root.Children, nil
}

// insertPath threads a file's slash-relative path into the tree, creating
// intermediate directory nodes as needed.
func insertPath(root *treeNode, rel string) {
	segs := strings.Split(rel, "/")
	cur := root
	for i, seg := range segs {
		isFile := i == len(segs)-1
		var child *treeNode
		for _, c := range cur.Children {
			if c.Name == seg && c.IsDir == !isFile {
				child = c
				break
			}
		}
		if child == nil {
			child = &treeNode{Name: seg, IsDir: !isFile, Rel: strings.Join(segs[:i+1], "/")}
			if isFile {
				child.URL = urlPrefix + rel
			}
			cur.Children = append(cur.Children, child)
		}
		cur = child
	}
}

// sortTree orders each level: directories before files, alphabetical within.
func sortTree(n *treeNode) {
	sort.Slice(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir
		}
		return a.Name < b.Name
	})
	for _, c := range n.Children {
		if c.IsDir {
			sortTree(c)
		}
	}
}

type pageData struct {
	Title      string
	Body       template.HTML
	Tree       []*treeNode
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
	tree, err := s.buildTree()
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
			Tree:   tree,
			Active: filepath.ToSlash(rel),
			CDN:    !s.opts.NoCDN,
		}
		return s.tmpl.Execute(f, data)
	})
}

// statusWriter records the response status code (and preserves http.Flusher so
// the live-reload SSE stream still flushes) for request logging.
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

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// logRequests wraps h to print one line per doc view: method, path, status, and
// latency. The live-reload SSE stream is skipped (it is long-lived and noisy).
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == liveReloadPath {
			h.ServeHTTP(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		start := time.Now()
		h.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK
		}
		fmt.Printf("mdserve: %s %s → %d (%s)\n", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Microsecond))
	})
}
