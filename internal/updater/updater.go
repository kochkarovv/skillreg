package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repoOwner = "kochkarovv"
	repoName  = "skillreg"
	latestURL = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"
)

// Release holds info about a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset is a single downloadable file in a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Version returns the tag without the "v" prefix.
func (r *Release) Version() string {
	return strings.TrimPrefix(r.TagName, "v")
}

// AssetURL returns the download URL for the current OS/arch, or empty string if not found.
func (r *Release) AssetURL() string {
	target := fmt.Sprintf("skillreg_%s_%s", runtime.GOOS, runtime.GOARCH)
	for _, a := range r.Assets {
		if strings.HasPrefix(a.Name, target) {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

// CheckLatest queries GitHub for the latest release.
// Returns the release if it is newer than currentVersion, nil otherwise.
// Uses a 2-second timeout to avoid blocking.
func CheckLatest(currentVersion string) (*Release, error) {
	if currentVersion == "dev" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}

	current := strings.TrimPrefix(currentVersion, "v")
	latest := rel.Version()

	if compareVersions(latest, current) > 0 {
		return &rel, nil
	}
	return nil, nil
}

// compareVersions compares two semver strings (without "v" prefix).
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func compareVersions(a, b string) int {
	ap := parseVersion(a)
	bp := parseVersion(b)
	for i := 0; i < 3; i++ {
		if ap[i] < bp[i] {
			return -1
		}
		if ap[i] > bp[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	parts := strings.SplitN(v, ".", 3)
	var result [3]int
	for i := 0; i < len(parts) && i < 3; i++ {
		n, _ := strconv.Atoi(parts[i])
		result[i] = n
	}
	return result
}

// Apply downloads the release asset for the current platform and replaces
// the binary at execPath. If execPath is empty, it uses os.Executable().
func Apply(rel *Release, execPath string) error {
	assetURL := rel.AssetURL()
	if assetURL == "" {
		return fmt.Errorf("no asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if execPath == "" {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return fmt.Errorf("locate executable: %w", err)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("resolve symlinks: %w", err)
		}
	}

	resp, err := http.Get(assetURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	newBinary, err := extractBinary(resp.Body)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	// Write new binary to temp file in the same directory
	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, "skillreg-update-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(newBinary); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp file: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("chmod: %w", err)
	}

	// Atomic-ish replace: rename current → .old, rename new → current
	oldPath := execPath + ".old"
	os.Remove(oldPath) // clean up from any previous failed update

	if err := os.Rename(execPath, oldPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backup current binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Restore the old binary
		os.Rename(oldPath, execPath)
		return fmt.Errorf("replace binary: %w", err)
	}

	// Clean up old binary (best-effort)
	os.Remove(oldPath)
	return nil
}

// extractBinary reads a tar.gz stream and returns the content of the "skillreg" file.
func extractBinary(r io.Reader) ([]byte, error) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := filepath.Base(hdr.Name)
		if name == "skillreg" || name == "skillreg.exe" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary not found in archive")
}
