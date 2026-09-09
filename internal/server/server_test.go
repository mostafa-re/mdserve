package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		`id="brand"`, `class="name">mdserve</span>`, `class="cwd" id="rootname"`, `id="sidefoot"`, // logo + title + cwd; version footer
		`viewBox="0 -960 960 960"`, `<path d="'+p+'"`, // Material Symbols; MS() wraps path data (guards blank-icon regression)
		"/vendor/marked.min.js", "/vendor/katex.min.css", "/vendor/hljs-dark.css", // embedded vendor bundle (offline)
		`id="filter"`, `id="b-theme"`, `id="b-font"`, `id="b-full"`, `id="b-out"`, `id="find-in"`, // controls incl. font + fullscreen
		`--sel-strong`, `--logo`, `data-theme="dark"`, `data-font="sans"`, // accents; themed logo; dark + sans defaults
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

func TestDirectDocumentPaths(t *testing.T) {
	srv, dir := newTestServer(t)
	mustWrite(t, filepath.Join(dir, "guide", "space #?.md"), "# Encoded\n")
	for _, target := range []string{"/guide", "/guide/", "/guide/intro.md", "/guide/space%20%23%3F.md"} {
		rec := get(t, srv, target)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "window.MDSERVE") {
			t.Errorf("%s: status %d, expected reader shell", target, rec.Code)
		}
		if rec.Header().Get("Location") != "" {
			t.Errorf("%s unexpectedly redirected", target)
		}
	}
	for _, target := range []string{"/missing", "/guide/missing.md"} {
		if rec := get(t, srv, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, rec.Code)
		}
	}
	rec := get(t, srv, "/raw?path="+url.QueryEscape("guide/space #?.md"))
	if rec.Code != http.StatusOK || rec.Body.String() != "# Encoded\n" {
		t.Fatalf("encoded filename: status %d, body %q", rec.Code, rec.Body.String())
	}
}

func TestFilePathsAndRanges(t *testing.T) {
	srv, dir := newTestServer(t)
	mustWrite(t, filepath.Join(dir, "guide", "image #.svg"), "<svg>test</svg>")
	for _, target := range []string{"/guide/image%20%23.svg", "/file?path=" + url.QueryEscape("guide/image #.svg")} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Header.Set("Range", "bytes=0-4")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusPartialContent || rec.Body.String() != "<svg>" {
			t.Errorf("%s: status %d, body %q", target, rec.Code, rec.Body.String())
		}
	}
}

func TestSymlinksCannotEscapeRoot(t *testing.T) {
	srv, dir := newTestServer(t)
	outside := t.TempDir()
	mustWrite(t, filepath.Join(outside, "secret.md"), "outside secret")
	for name, target := range map[string]string{
		"leak.md":   filepath.Join(outside, "secret.md"),
		"leak":      outside,
		"inside.md": filepath.Join("guide", "intro.md"),
	} {
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
	for _, target := range []string{
		"/raw?path=leak.md", "/file?path=leak.md", "/leak.md",
		"/raw?path=leak/secret.md", "/file?path=leak/secret.md", "/leak/secret.md", "/leak/",
	} {
		if rec := get(t, srv, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, rec.Code)
		}
	}
	if rec := get(t, srv, "/raw?path=inside.md"); rec.Code != http.StatusOK {
		t.Errorf("relative symlink inside root: status %d, want 200", rec.Code)
	}
}

func TestBuildRejectsOutsideSymlinks(t *testing.T) {
	for _, name := range []string{"leak.md", "leak.txt"} {
		t.Run(name, func(t *testing.T) {
			srv, dir := newTestServer(t)
			outside := filepath.Join(t.TempDir(), "secret")
			mustWrite(t, outside, "outside secret")
			if err := os.Symlink(outside, filepath.Join(dir, name)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			if err := srv.BuildStatic(t.TempDir()); err == nil {
				t.Fatal("build must reject symlinks outside its root")
			}
		})
	}
}

func TestDefaultDocEscaped(t *testing.T) {
	value := "quotes\"\\</script><script>alert(1)</script>\u2028.md"
	srv, err := NewServer(Options{Dir: t.TempDir(), DefaultDoc: value, Version: "<img src=x>"})
	if err != nil {
		t.Fatal(err)
	}
	config := strings.SplitN(strings.SplitN(srv.page, "defaultDoc:", 2)[1], "};</script>", 2)[0]
	var got string
	if err := json.Unmarshal([]byte(config), &got); err != nil || got != value {
		t.Fatalf("defaultDoc failed to round trip: %q, %v", got, err)
	}
	if strings.Contains(config, "</script>") || strings.Contains(srv.page, "<img src=x>") {
		t.Fatal("configuration injected HTML")
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

func TestFileServed(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# H\n")
	mustWrite(t, filepath.Join(dir, "img", "logo.png"), "\x89PNG\r\n\x1a\nfake")
	mustWrite(t, filepath.Join(dir, "d.svg"), `<svg xmlns="http://www.w3.org/2000/svg"/>`)
	srv, err := NewServer(Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path, wantCT string
	}{
		{"img/logo.png", "image/png"},
		{"d.svg", "image/svg+xml"},
	}
	for _, c := range cases {
		rec := get(t, srv, "/file?path="+c.path)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200", c.path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, c.wantCT) {
			t.Errorf("%s content-type = %q, want %q", c.path, ct, c.wantCT)
		}
	}
	for _, bad := range []string{"../../etc/hosts", "nope.png", "img"} {
		if get(t, srv, "/file?path="+bad).Code == http.StatusOK {
			t.Errorf("path %q should not 200", bad)
		}
	}
}

func TestFaviconServed(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, p := range []string{"/favicon.svg", "/favicon.ico"} {
		rec := get(t, srv, p)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 (no favicon probe 404)", p, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
			t.Errorf("%s content-type = %q, want image/svg+xml", p, ct)
		}
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
	srv, dir := newTestServer(t)
	mustWrite(t, filepath.Join(dir, "img", "pic.png"), "\x89PNG\r\n\x1a\nfake")
	out := t.TempDir()
	if err := srv.BuildStatic(out); err != nil {
		t.Fatalf("BuildStatic: %v", err)
	}
	for _, rel := range []string{"README.md.html", filepath.Join("guide", "intro.md.html"), filepath.Join("vendor", "marked.min.js"), filepath.Join("img", "pic.png")} {
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
