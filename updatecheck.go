package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// updateInfo is the JSON returned by GET /api/update-check. The reader fetches
// it once at load and shows a banner when Update is true.
type updateInfo struct {
	Current  string `json:"current"`
	Latest   string `json:"latest,omitempty"`
	Update   bool   `json:"update"`
	Dev      bool   `json:"dev,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// updateCheckTTL caps how often the server reaches GitHub: at most once per
// window, the rest served from an in-memory + on-disk cache. Keeps the
// offline-first promise — a launch never spams the network.
const updateCheckTTL = 24 * time.Hour

// updateCheckTimeout bounds the single GitHub call so a slow network can't stall
// the reader's banner fetch.
const updateCheckTimeout = 5 * time.Second

// handleUpdateCheck reports whether a newer release exists. It never blocks the
// server on the network beyond updateCheckTimeout, never errors out to the
// client, and is a no-op (no outbound call) for dev builds or when disabled.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	cur := versionString()
	if !s.opts.UpdateCheck {
		writeJSON(w, updateInfo{Current: cur, Disabled: true})
		return
	}
	if !isReleaseVersion(cur) {
		// dev / dev+commit / pseudo-version: nothing to update to, stay offline.
		writeJSON(w, updateInfo{Current: cur, Dev: true})
		return
	}
	latest, ok := cachedLatestTag()
	if !ok {
		// network/parse failure — fail silent, just say "no update".
		writeJSON(w, updateInfo{Current: cur})
		return
	}
	writeJSON(w, updateInfo{Current: cur, Latest: latest, Update: latest != cur})
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
