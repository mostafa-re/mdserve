package main

import (
	"net"
	"testing"
)

// --- update indicator -----------------------------------------------------

// The indicator must stay silent (and make no network call) when disabled, and
// on a dev build — which the test binary always is (no -ldflags version).
func TestUpdateIndicatorSilent(t *testing.T) {
	if got := updateIndicator(false); got != "" {
		t.Errorf("disabled indicator = %q, want empty", got)
	}
	if got := updateIndicator(true); got != "" {
		t.Errorf("dev-build indicator = %q, want empty (no release to compare, no network)", got)
	}
}

// versionString only ever yields a clean tag (vX.Y.Z) or a dev form
// (dev / dev+<commit> / *-dirty); isReleaseVersion separates the two.
func TestIsReleaseVersion(t *testing.T) {
	cases := map[string]bool{
		"v0.5.1":           true,
		"v1.2.3":           true,
		"dev":              false,
		"dev+abc123":       false,
		"v0.5.1-dirty":     false,
		"dev+abc123-dirty": false,
		"":                 false,
	}
	for v, want := range cases {
		if got := isReleaseVersion(v); got != want {
			t.Errorf("isReleaseVersion(%q) = %v, want %v", v, got, want)
		}
	}
}

// the indicator must never advertise a downgrade — only a strictly-newer release.
func TestIsNewer(t *testing.T) {
	cases := []struct {
		b, a string
		want bool
	}{
		{"v0.6.1", "v0.6.0", true},        // newer patch
		{"v0.6.0", "v0.6.1", false},       // older — never suggest a downgrade
		{"v0.6.0", "v0.6.0", false},       // equal
		{"v1.0.0", "v0.9.9", true},        // newer major
		{"v0.10.0", "v0.9.0", true},       // numeric, not lexical (10 > 9)
		{"v0.6.0", "v0.6.0+dirty", false}, // build metadata stripped → equal
		{"v0.6.1", "v0.6.0-rc1", true},    // prerelease stripped
		{"garbage", "v0.6.0", false},      // unparseable → false
		{"v0.6.1", "weird", false},
	}
	for _, c := range cases {
		if got := isNewer(c.b, c.a); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.b, c.a, got, c.want)
		}
	}
}

// --- listen / defaults ----------------------------------------------------

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
