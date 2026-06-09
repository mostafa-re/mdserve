package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// serveLiveReload streams an SSE event whenever a watched .md file changes; the
// page's injected client reloads on the first message.
func (s *Server) serveLiveReload(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.clients[ch] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.clients, ch)
		s.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	fmt.Fprint(w, "retry: 1000\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ch:
			fmt.Fprint(w, "data: reload\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) broadcast() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.clients {
		select {
		case ch <- struct{}{}:
		default: // a reload is already pending for this client
		}
	}
}

// startWatch polls the docs tree for mtime changes every 500ms and broadcasts a
// reload when the newest mtime advances. Polling (vs fsnotify) keeps the tool
// dependency-free and behaves identically on every OS; the doc tree is small.
func (s *Server) startWatch() {
	go func() {
		last := s.newestMtime()
		for {
			time.Sleep(500 * time.Millisecond)
			if m := s.newestMtime(); m > last {
				last = m
				s.broadcast()
			}
		}
	}()
}

// newestMtime returns the newest modification time (UnixNano) among .md files.
func (s *Server) newestMtime() int64 {
	var newest int64
	_ = filepath.WalkDir(s.docDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
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
		if info, err := d.Info(); err == nil {
			if m := info.ModTime().UnixNano(); m > newest {
				newest = m
			}
		}
		return nil
	})
	return newest
}
