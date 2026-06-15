package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// runUpdate replaces the running binary with the latest released build for this
// OS/arch from GitHub Releases. `mdserve update --check` only reports whether a
// newer release exists.
func runUpdate(args []string) {
	check := false
	for _, a := range args {
		if a == "--check" || a == "-check" {
			check = true
		}
	}
	cur := versionString()
	latest, err := latestTag()
	if err != nil {
		fatal(fmt.Errorf("checking latest release: %w", err))
	}
	fmt.Printf("mdserve: current %s\n", cur)
	fmt.Printf("mdserve: latest  %s\n", latest)
	if latest == cur {
		fmt.Println("mdserve: already up to date")
		return
	}
	if check {
		fmt.Printf("mdserve: update available — run `mdserve update`\n")
		return
	}
	bin, err := downloadReleaseBinary(latest)
	if err != nil {
		fatal(fmt.Errorf("downloading %s: %w", latest, err))
	}
	if err := replaceExecutable(bin); err != nil {
		fatal(fmt.Errorf("installing update: %w", err))
	}
	fmt.Printf("mdserve: updated %s → %s\n", cur, latest)
}

const updateUA = "mdserve-selfupdate"

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", updateUA)
	return (&http.Client{Timeout: 60 * time.Second}).Do(req)
}

// latestTag returns the tag_name of the latest GitHub release (60s timeout).
func latestTag() (string, error) {
	return latestTagCtx(60 * time.Second)
}

// latestTagCtx is latestTag with a caller-chosen timeout. The launch-time update
// check (updatecheck.go) uses a short one so a slow network can't stall the page.
func latestTagCtx(timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/"+repoSlug+"/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", updateUA)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github api: %s", resp.Status)
	}
	var r struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.TagName == "" {
		return "", fmt.Errorf("no releases found")
	}
	return r.TagName, nil
}

// assetFor returns the release asset filename and the binary name inside it for
// this OS/arch — matching the names produced by .github/workflows/release.yml.
func assetFor(tag string) (asset, binName string) {
	binName = "mdserve"
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		binName = "mdserve.exe"
		ext = "zip"
	}
	return fmt.Sprintf("mdserve_%s_%s_%s.%s", tag, runtime.GOOS, runtime.GOARCH, ext), binName
}

// downloadReleaseBinary fetches the release archive for this platform and returns
// the extracted binary bytes.
func downloadReleaseBinary(tag string) ([]byte, error) {
	asset, binName := assetFor(tag)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repoSlug, tag, asset)
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", asset, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(asset, ".zip") {
		return extractFromZip(data, binName)
	}
	return extractFromTarGz(data, binName)
}

func extractFromTarGz(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == name {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

func extractFromZip(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

// replaceExecutable swaps the running binary for newBin. It writes a temp file in
// the same directory (so the rename is atomic on the same filesystem) and renames
// it over the current executable. On Windows the running file can't be
// overwritten, so it is moved aside to <exe>.old first.
func replaceExecutable(newBin []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".mdserve-update-*")
	if err != nil {
		return fmt.Errorf("%w (need write access to %s — re-run with sudo or reinstall)", err, dir)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if _, err := tmp.Write(newBin); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			cleanup()
			return err
		}
	}
	if err := os.Rename(tmpName, exe); err != nil {
		cleanup()
		return fmt.Errorf("%w (need write access to %s)", err, dir)
	}
	return nil
}
