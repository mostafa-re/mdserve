package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- exclude --------------------------------------------------------------

func TestIsExcluded(t *testing.T) {
	srv := &Server{exclude: []string{"node_modules", "*.private.md", "drafts/*", "build/out.md"}}
	cases := []struct {
		rel  string
		want bool
	}{
		{"node_modules", true},               // exact dir name
		{"node_modules/pkg/readme.md", true}, // subtree (segment match)
		{"a/b/node_modules/x.md", true},      // segment at depth
		{"secret.private.md", true},          // glob by filename
		{"notes/plan.private.md", true},      // glob by filename at depth
		{"drafts/x.md", true},                // path glob, direct child
		{"drafts/sub/x.md", false},           // path glob is one level only
		{"build/out.md", true},               // full-path match
		{"build/other.md", false},            // full-path match is specific
		{"README.md", false},                 // unrelated
		{"guide/intro.md", false},            // unrelated nested
		{"node_modules_x/readme.md", false},  // not a whole-segment match
		{"Node_Modules", true},               // case-insensitive: dir name
		{"a/NODE_MODULES/x.md", true},        // case-insensitive: segment at depth
		{"notes/Plan.PRIVATE.MD", true},      // case-insensitive: filename glob
		{"DRAFTS/x.md", true},                // case-insensitive: path glob
		{"Build/Out.md", true},               // case-insensitive: full-path match
	}
	for _, c := range cases {
		if got := srv.isExcluded(c.rel); got != c.want {
			t.Errorf("isExcluded(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
}

func TestNewServerRejectsBadExcludeGlob(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# H\n")
	if _, err := NewServer(Options{Dir: dir, Exclude: []string{"["}}); err == nil {
		t.Fatal("expected NewServer to reject an invalid glob pattern")
	}
}

func excludeTree(t *testing.T) (*Server, string) {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "README.md"), "# Home\n")
	mustWrite(t, filepath.Join(dir, "guide", "intro.md"), "# Intro\n")
	mustWrite(t, filepath.Join(dir, "secret", "plan.md"), "# Secret\n")
	mustWrite(t, filepath.Join(dir, "secret", "data.bin"), "x")
	mustWrite(t, filepath.Join(dir, "notes", "draft.private.md"), "# Draft\n")
	srv, err := NewServer(Options{Dir: dir, Exclude: []string{"secret", "*.private.md"}})
	if err != nil {
		t.Fatal(err)
	}
	return srv, dir
}

func TestExcludeHiddenFromAPIs(t *testing.T) {
	srv, _ := excludeTree(t)

	tree := get(t, srv, "/api/tree").Body.String()
	if strings.Contains(tree, "secret") {
		t.Errorf("/api/tree leaked excluded dir 'secret':\n%s", tree)
	}
	if strings.Contains(tree, "private") {
		t.Errorf("/api/tree leaked excluded file '*.private.md':\n%s", tree)
	}
	if !strings.Contains(tree, "guide/intro.md") {
		t.Errorf("/api/tree dropped a non-excluded file:\n%s", tree)
	}

	var poll map[string]float64
	if err := json.Unmarshal(get(t, srv, "/api/poll").Body.Bytes(), &poll); err != nil {
		t.Fatal(err)
	}
	if _, ok := poll["secret/plan.md"]; ok {
		t.Errorf("/api/poll included excluded secret/plan.md: %v", poll)
	}
	if _, ok := poll["notes/draft.private.md"]; ok {
		t.Errorf("/api/poll included excluded *.private.md: %v", poll)
	}
	if _, ok := poll["guide/intro.md"]; !ok {
		t.Errorf("/api/poll dropped a non-excluded file: %v", poll)
	}
}

func TestExcludeBlocksDirectAccess(t *testing.T) {
	srv, _ := excludeTree(t)
	for _, p := range []string{"/raw?path=secret/plan.md", "/raw?path=notes/draft.private.md", "/file?path=secret/data.bin", "/secret/plan.md", "/secret/", "/notes/draft.private.md"} {
		if get(t, srv, p).Code == http.StatusOK {
			t.Errorf("%s reached an excluded path (got 200)", p)
		}
	}
	if get(t, srv, "/raw?path=guide/intro.md").Code != http.StatusOK {
		t.Errorf("a non-excluded doc should still be served")
	}
}

func TestExcludeDirectoryPatternBlocksDescendants(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "drafts", "sub", "plan.md"), "# Private\n")
	srv, err := NewServer(Options{Dir: dir, Exclude: []string{"drafts/*"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/drafts/sub/plan.md", "/raw?path=drafts/sub/plan.md", "/file?path=drafts/sub/plan.md"} {
		if rec := get(t, srv, target); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", target, rec.Code)
		}
	}
}

func TestExcludeAbsentFromStaticBuild(t *testing.T) {
	srv, _ := excludeTree(t)
	out := t.TempDir()
	if err := srv.BuildStatic(out); err != nil {
		t.Fatalf("BuildStatic: %v", err)
	}
	for _, gone := range []string{
		filepath.Join("secret", "plan.md.html"),
		filepath.Join("secret", "data.bin"),
		filepath.Join("notes", "draft.private.md.html"),
	} {
		if _, err := os.Stat(filepath.Join(out, gone)); err == nil {
			t.Errorf("static build included excluded %s", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "README.md.html")); err != nil {
		t.Errorf("static build dropped a non-excluded doc: %v", err)
	}
}

// --- RTL ------------------------------------------------------------------

func TestRTLAssetsInReader(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, want := range []string{
		`@font-face`,
		`font-family:"Vazirmatn"`,
		`font-family:"Noto Sans Arabic"`,
		`/vendor/fonts/Vazirmatn-arabic.woff2`,
		`/vendor/fonts/NotoSansArabic-arabic.woff2`,
		`unicode-range:`,
		`function tagDir(`,
		`setAttribute("dir","auto")`,
	} {
		if !strings.Contains(srv.page, want) {
			t.Errorf("reader page missing RTL asset %q", want)
		}
	}
}

func TestRTLFontsEmbedded(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, f := range []string{"/vendor/fonts/Vazirmatn-arabic.woff2", "/vendor/fonts/NotoSansArabic-arabic.woff2"} {
		rec := get(t, srv, f)
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d, want 200 (font must be embedded)", f, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "font/woff2") {
			t.Errorf("%s content-type = %q, want font/woff2", f, ct)
		}
	}
}

func TestRTLInStaticBuild(t *testing.T) {
	srv, _ := newTestServer(t)
	out := t.TempDir()
	if err := srv.BuildStatic(out); err != nil {
		t.Fatalf("BuildStatic: %v", err)
	}
	html, err := os.ReadFile(filepath.Join(out, "README.md.html"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(html)
	for _, want := range []string{
		`@font-face`,
		`font-family:"Vazirmatn"`,
		`vendor/fonts/Vazirmatn-arabic.woff2`,
		`setAttribute("dir","auto")`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("static build page missing RTL asset %q", want)
		}
	}
	// the woff2 files must be copied alongside for the offline build
	if _, err := os.Stat(filepath.Join(out, "vendor", "fonts", "Vazirmatn-arabic.woff2")); err != nil {
		t.Errorf("static build missing copied Arabic font: %v", err)
	}
}

// the web update banner was replaced by a CLI indicator — its markup, route, and
// client wiring must be fully gone from the reader.
func TestUpdateBannerRemoved(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, gone := range []string{`id="update-banner"`, `/api/update-check`, `mdserve.update.dismissed`} {
		if strings.Contains(srv.page, gone) {
			t.Errorf("reader page still references removed update banner %q", gone)
		}
	}
	if get(t, srv, "/api/update-check").Code != http.StatusNotFound {
		t.Error("/api/update-check should be gone (404)")
	}
}

// navigation (tree rows + outline entries) must carry dir="auto" so RTL-named
// items render right-to-left.
func TestNavRTLWiring(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, want := range []string{
		`row.setAttribute("dir","auto")`, // tree rows
		`a.setAttribute("dir","auto")`,   // outline entries
		`id="crumb" dir="auto"`,          // breadcrumb
		`padding-inline-start`,           // outline indent flips for RTL
	} {
		if !strings.Contains(srv.page, want) {
			t.Errorf("reader page missing nav-RTL wiring %q", want)
		}
	}
}
