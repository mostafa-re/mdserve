package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// updateCheckTTL caps how often we reach GitHub: at most once per window, the
// rest served from an in-memory + on-disk cache. Keeps the offline-first promise
// — launching never spams the network.
const updateCheckTTL = 24 * time.Hour

// updateCheckTimeout bounds the single GitHub call so a slow network can't stall
// the CLI for long.
const updateCheckTimeout = 5 * time.Second

// updateIndicator returns a short, colored status comparing this build to the
// latest GitHub release: a green "latest" when current, a yellow "update
// available" line otherwise. It is empty (no output, no network) when disabled,
// on a dev build, or when the check can't reach GitHub — so it stays silent and
// offline-friendly. Shown by `mdserve version` and at serve startup.
func updateIndicator(enabled bool) string {
	if !enabled {
		return ""
	}
	cur := versionString()
	if !isReleaseVersion(cur) {
		return ""
	}
	latest, ok := cachedLatestTag()
	if !ok {
		return ""
	}
	// Only a strictly-newer release is an update — never suggest a downgrade when
	// this build is ahead of the latest published release.
	if isNewer(latest, cur) {
		return paint("33", "● update available: "+latest+" — run: mdserve update")
	}
	return paint("32", "● latest")
}

// isNewer reports whether release tag b is strictly newer than a (both vX.Y.Z).
// Dependency-free numeric compare; on any parse ambiguity it returns false, so we
// never advertise a downgrade.
func isNewer(b, a string) bool {
	bp, okb := parseSemver(b)
	ap, oka := parseSemver(a)
	if !okb || !oka {
		return false
	}
	for i := range bp {
		if bp[i] != ap[i] {
			return bp[i] > ap[i]
		}
	}
	return false
}

// parseSemver extracts major.minor.patch from a tag like "v1.2.3", dropping a "v"
// prefix and any -prerelease / +build suffix. ok is false if it isn't numeric
// dotted (≤3 parts).
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return [3]int{}, false
	}
	var out [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [3]int{}, false
		}
		out[i] = n
	}
	return out, true
}

// isReleaseVersion reports whether v is a clean release tag (vX.Y.Z). Dev builds
// report "dev" / "dev+<commit>" (versionString already collapses Go's synthetic
// pseudo-versions to that form), so a leading "v" is enough to tell them apart.
func isReleaseVersion(v string) bool {
	return strings.HasPrefix(v, "v") && !strings.Contains(v, "+") && !strings.Contains(v, "-dirty")
}

var (
	updMu     sync.Mutex
	updTag    string
	updStamp  time.Time // when updTag was fetched or loaded from disk
	updTried  time.Time // last network attempt (success OR failure) — throttle
	updLoaded bool      // whether the on-disk cache has been consulted this run
)

// cachedLatestTag returns the latest release tag, served from the in-memory or
// on-disk cache when fresh (< updateCheckTTL), otherwise fetched once and cached.
// Network attempts are throttled to once per TTL window even when they fail, so
// an offline release binary never re-hits GitHub on every page load. ok is false
// on a fetch failure with no usable cache.
func cachedLatestTag() (string, bool) {
	updMu.Lock()
	defer updMu.Unlock()

	if !updLoaded {
		updLoaded = true
		if tag, at, ok := readUpdateCache(); ok {
			updTag, updStamp = tag, at
		}
	}
	if updTag != "" && time.Since(updStamp) < updateCheckTTL {
		return updTag, true
	}
	// throttle: at most one network attempt per TTL window, success or failure.
	if !updTried.IsZero() && time.Since(updTried) < updateCheckTTL {
		if updTag != "" {
			return updTag, true // serve a stale tag rather than nothing
		}
		return "", false
	}
	updTried = time.Now()
	tag, err := latestTagCtx(updateCheckTimeout)
	if err != nil {
		if updTag != "" {
			return updTag, true
		}
		return "", false
	}
	updTag, updStamp = tag, time.Now()
	writeUpdateCache(tag, updStamp)
	return tag, true
}

// updateCacheFile is the on-disk cache shape under os.UserCacheDir()/mdserve.
type updateCacheFile struct {
	Tag       string    `json:"tag"`
	CheckedAt time.Time `json:"checked_at"`
}

func updateCachePath() (string, bool) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(dir, "mdserve", "update.json"), true
}

func readUpdateCache() (string, time.Time, bool) {
	p, ok := updateCachePath()
	if !ok {
		return "", time.Time{}, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", time.Time{}, false
	}
	var c updateCacheFile
	if err := json.Unmarshal(b, &c); err != nil || c.Tag == "" {
		return "", time.Time{}, false
	}
	return c.Tag, c.CheckedAt, true
}

func writeUpdateCache(tag string, at time.Time) {
	p, ok := updateCachePath()
	if !ok {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(updateCacheFile{Tag: tag, CheckedAt: at})
	if err != nil {
		return
	}
	_ = os.WriteFile(p, b, 0o644)
}
