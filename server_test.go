package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer writes a small doc tree and returns a Server rooted at it.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# Home\n\nwelcome\n")
	mustWrite(t, filepath.Join(dir, "guide", "intro.md"), "# Intro\n\n| a | b |\n|---|---|\n| 1 | 2 |\n")
	srv, err := NewServer(Options{Dir: dir, DefaultDoc: "README.md"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv, dir
}

func mustWrite(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func get(t *testing.T, srv *Server, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestNewServer_RejectsNonDir(t *testing.T) {
	if _, err := NewServer(Options{Dir: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

// TestServesReaderShell asserts "/" returns the ported single-page reader with
// the three required adaptations: an mdserve brand header, Google Material
// Symbols (the 0 -960 960 960 viewBox), and no bookmark/reading-marker.
func TestServesReaderShell(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"<title>mdserve</title>",
		`id="brand"`, `class="name">mdserve</span>`, `class="ver"`, `class="cwd" id="rootname"`, // logo + title + version + cwd
		`viewBox="0 -960 960 960"`, `<path d="'+p+'"`, // Material Symbols; MS() wraps path data (guards blank-icon regression)
		"/vendor/marked.min.js", "/vendor/katex.min.css", // embedded vendor bundle (offline)
		`id="filter"`, `id="b-theme"`, `id="b-font"`, `id="b-full"`, `id="b-out"`, `id="find-in"`, // controls incl. font + fullscreen
		`--sel-strong`, `--logo`, `data-theme="dark"`, `data-font="serif"`, // accents; themed logo; dark + serif defaults
		"window.MDSERVE",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("reader shell missing %q", want)
		}
	}
	for _, gone := range []string{"readmark", "bookmark", "/api/state"} {
		if strings.Contains(body, gone) {
			t.Errorf("reader shell still references removed feature %q", gone)
		}
	}
}

// TestReloadAndDefaultSentinels checks the per-server substitutions.
func TestReloadAndDefaultSentinels(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# H\n")
	on, _ := NewServer(Options{Dir: dir, DefaultDoc: "guide/x.md", Reload: true})
	off, _ := NewServer(Options{Dir: dir, Reload: false})
	if !strings.Contains(on.page, "reload:true") {
		t.Error("reload:true not injected when Reload is on")
	}
	if !strings.Contains(on.page, `defaultDoc:"guide/x.md"`) {
		t.Error("default doc not injected")
	}
	if !strings.Contains(off.page, "reload:false") {
		t.Error("reload:false not injected when Reload is off")
	}
	if strings.Contains(on.page, "__RELOAD__") || strings.Contains(on.page, "__DEFAULT__") {
		t.Error("sentinels left unsubstituted")
	}
}

// TestAPITree decodes /api/tree and checks the directories-first nesting.
func TestAPITree(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/api/tree")
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want json", ct)
	}
	var resp struct {
		Root string    `json:"root"`
		Tree []apiNode `json:"tree"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tree) != 2 {
		t.Fatalf("top-level nodes = %d, want 2", len(resp.Tree))
	}
	if resp.Tree[0].Type != "dir" || resp.Tree[0].Name != "guide" {
		t.Fatalf("first node = %+v, want dir guide", resp.Tree[0])
	}
	if len(resp.Tree[0].Children) != 1 || resp.Tree[0].Children[0].Relpath != "guide/intro.md" {
		t.Fatalf("guide children = %+v, want guide/intro.md", resp.Tree[0].Children)
	}
	if resp.Tree[1].Type != "file" || resp.Tree[1].Relpath != "README.md" {
		t.Fatalf("second node = %+v, want README.md leaf", resp.Tree[1])
	}
}

func TestRawServesMarkdown(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/raw?path=guide/intro.md")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/markdown") {
		t.Fatalf("content-type = %q, want markdown", ct)
	}
	if !strings.Contains(rec.Body.String(), "# Intro") {
		t.Errorf("raw markdown missing heading:\n%s", rec.Body.String())
	}
}

func TestRawPathTraversalBlocked(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, bad := range []string{"../../etc/passwd.md", "/etc/passwd.md", "guide/intro.txt", "guide"} {
		rec := get(t, srv, "/raw?path="+bad)
		if rec.Code == http.StatusOK {
			t.Errorf("path %q returned 200, want non-200", bad)
		}
	}
}

func TestAPIPoll(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/api/poll")
	var m map[string]float64
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := m["README.md"]; !ok {
		t.Errorf("poll map missing README.md: %v", m)
	}
	if _, ok := m["guide/intro.md"]; !ok {
		t.Errorf("poll map missing guide/intro.md: %v", m)
	}
}

func TestVendorAssetServed(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := get(t, srv, "/vendor/marked.min.js")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("content-type = %q, want javascript", ct)
	}
	if get(t, srv, "/vendor/does-not-exist.js").Code != http.StatusNotFound {
		t.Error("unknown vendor asset should 404")
	}
	if get(t, srv, "/vendor/fonts/KaTeX_Main-Regular.woff2").Code != http.StatusOK {
		t.Error("embedded KaTeX font should be served")
	}
}

// TestBuildStaticOffline renders the static site and checks each doc is a
// standalone page that references the copied vendor bundle at the right depth.
func TestBuildStaticOffline(t *testing.T) {
	srv, _ := newTestServer(t)
	out := t.TempDir()
	if err := srv.BuildStatic(out); err != nil {
		t.Fatalf("BuildStatic: %v", err)
	}
	for _, rel := range []string{"README.md.html", filepath.Join("guide", "intro.md.html"), filepath.Join("vendor", "marked.min.js")} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
	root, err := os.ReadFile(filepath.Join(out, "README.md.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(root), "<h1") || !strings.Contains(string(root), "Home") {
		t.Errorf("README.md.html missing rendered heading")
	}
	if !strings.Contains(string(root), `href="vendor/hljs-theme.css"`) {
		t.Errorf("root page should reference vendor at depth 0")
	}
	nested, err := os.ReadFile(filepath.Join(out, "guide", "intro.md.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nested), "<table>") {
		t.Errorf("intro.md.html missing rendered table")
	}
	if !strings.Contains(string(nested), `href="../vendor/hljs-theme.css"`) {
		t.Errorf("nested page should reference vendor at depth 1 (../)")
	}
}

func TestDefaultDirIsWorkingDir(t *testing.T) {
	// With no --dir, a bare `mdserve` serves the current working directory, not a
	// hardcoded "docs" subdir — so `cd any-repo && mdserve` just works.
	if defaultDir != "." {
		t.Fatalf("defaultDir = %q, want %q so a bare `mdserve` serves the cwd", defaultDir, ".")
	}
}

func TestDefaultAddrIsLoopback(t *testing.T) {
	// The default listen addr must be a concrete loopback addr, never a wildcard
	// (":8080"). A wildcard bind SUCCEEDS even when another process already holds
	// 127.0.0.1:<port>, so the free-port fallback never fires — yet the advertised
	// http://127.0.0.1:<port>/ URL is then served by that other process. Binding
	// loopback makes net.Listen fail with EADDRINUSE so listen() falls back to a
	// genuinely free port.
	host, _, err := net.SplitHostPort(defaultAddr)
	if err != nil {
		t.Fatalf("SplitHostPort(%q): %v", defaultAddr, err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("default addr host = %q, want a loopback IP so the free-port fallback engages when the port is busy", host)
	}
}

func TestListenFreePortFallback(t *testing.T) {
	// Occupy a port, then ask listen() for it — it must fall back to a free one.
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	ln, shown, err := listen(busy.Addr().String())
	if err != nil {
		t.Fatalf("listen fallback: %v", err)
	}
	defer func() { _ = ln.Close() }()
	if shown == busy.Addr().String() {
		t.Fatalf("expected a different free port, got the busy one %s", shown)
	}
}
