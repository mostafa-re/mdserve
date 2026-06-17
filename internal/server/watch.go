package server

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// flattenMtimes returns {slash-rel-path: mtime-unix-seconds} for every .md file
// under docDir. The browser polls /api/poll and compares this map to detect
// added/removed/changed files and live-reload — an mtime poll (vs fsnotify)
// keeps the tool dependency-free and identical on every OS; the doc tree is
// small. Hidden dirs and the static-build output (_site) are skipped.
func (s *Server) flattenMtimes() map[string]float64 {
	out := map[string]float64{}
	_ = filepath.WalkDir(s.docDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") || d.Name() == "_site" {
				return fs.SkipDir
			}
			if rel, e := filepath.Rel(s.docDir, p); e == nil {
				if r := filepath.ToSlash(rel); r != "." && s.isExcluded(r) {
					return fs.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		rel, e := filepath.Rel(s.docDir, p)
		if e != nil {
			return nil
		}
		if s.isExcluded(filepath.ToSlash(rel)) {
			return nil
		}
		if info, e := d.Info(); e == nil {
			out[filepath.ToSlash(rel)] = float64(info.ModTime().UnixNano()) / 1e9
		}
		return nil
	})
	return out
}
