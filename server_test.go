package main

import (
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

func TestNewServer_RejectsNonDir(t *testing.T) {
	if _, err := NewServer(Options{Dir: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

func TestRootRedirectsToDefaultDoc(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/docs/README.md" {
		t.Fatalf("Location = %q, want /docs/README.md", loc)
	}
}

func TestRendersMarkdown(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/guide/intro.md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<h1") || !strings.Contains(body, "Intro") {
		t.Errorf("missing rendered heading:\n%s", body)
	}
	if !strings.Contains(body, "<table>") {
		t.Errorf("table extension not rendered")
	}
}

func TestPathTraversalBlocked(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	// %2e%2e escapes are normalized by net/http; assert an out-of-root path is not 200.
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/../../../etc/passwd.md", nil))
	if rec.Code == http.StatusOK {
		t.Fatalf("traversal returned 200: %s", rec.Body.String())
	}
}

func TestLiveReloadInjectedOnlyWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# H\n")
	on, _ := NewServer(Options{Dir: dir, LiveReload: true})
	off, _ := NewServer(Options{Dir: dir, LiveReload: false})
	if !strings.Contains(render(on), "/__mdserve_reload") {
		t.Error("live-reload script missing when enabled")
	}
	if strings.Contains(render(off), "/__mdserve_reload") {
		t.Error("live-reload script present when disabled")
	}
}

func render(s *Server) string {
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs/README.md", nil))
	return rec.Body.String()
}

func TestBuildStatic(t *testing.T) {
	srv, _ := newTestServer(t)
	out := t.TempDir()
	if err := srv.BuildStatic(out); err != nil {
		t.Fatalf("BuildStatic: %v", err)
	}
	for _, rel := range []string{"README.md.html", filepath.Join("guide", "intro.md.html")} {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}
}

func TestDefaultAddrIsLoopback(t *testing.T) {
	// The default listen addr must be a concrete loopback addr, never a wildcard
	// (":8080"). A wildcard bind SUCCEEDS even when another process already holds
	// 127.0.0.1:<port>, so the free-port fallback never fires — yet the advertised
	// http://127.0.0.1:<port>/ URL is then served by that other process (observed
	// in the wild: a local proxy on :8080 answering 400 to every request). Binding
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
