package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"0.2.0", "0.2.0", 0},
		{"1.0.0", "0.9.9", 1},
		{"0.10.0", "0.9.0", 1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.99.99", 1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCheckLatest_DevVersionSkips(t *testing.T) {
	rel, err := CheckLatest("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Error("expected nil release for dev version")
	}
}

func TestRelease_AssetURL(t *testing.T) {
	rel := Release{
		TagName: "v0.3.0",
		Assets: []Asset{
			{Name: "skillreg_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64.tar.gz"},
			{Name: "skillreg_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64.tar.gz"},
			{Name: "skillreg_darwin_amd64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_amd64.tar.gz"},
			{Name: "skillreg_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64.zip"},
		},
	}
	url := rel.AssetURL()
	if url == "" {
		t.Fatalf("expected a matching asset URL for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	found := false
	for _, a := range rel.Assets {
		if a.BrowserDownloadURL == url {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("returned URL %q not in asset list", url)
	}
}

func TestRelease_AssetURL_NoMatch(t *testing.T) {
	rel := Release{
		TagName: "v0.3.0",
		Assets: []Asset{
			{Name: "skillreg_fakeos_fakearch.tar.gz", BrowserDownloadURL: "https://example.com/fake.tar.gz"},
		},
	}
	url := rel.AssetURL()
	if url != "" {
		t.Errorf("expected empty URL for non-matching platform, got %q", url)
	}
}

func TestRelease_Version(t *testing.T) {
	rel := Release{TagName: "v1.2.3"}
	if v := rel.Version(); v != "1.2.3" {
		t.Errorf("Version() = %q, want %q", v, "1.2.3")
	}
}

func createTestTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	hdr := &tar.Header{
		Name: name,
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	return buf.Bytes()
}

func TestApply(t *testing.T) {
	newContent := []byte("#!/bin/sh\necho new-version\n")
	archiveData := createTestTarGz(t, "skillreg", newContent)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	currentBin := filepath.Join(tmpDir, "skillreg")
	os.WriteFile(currentBin, []byte("old-version"), 0755)

	rel := &Release{
		TagName: "v0.3.0",
		Assets: []Asset{
			{
				Name:               "skillreg_" + runtime.GOOS + "_" + runtime.GOARCH + ".tar.gz",
				BrowserDownloadURL: srv.URL + "/skillreg.tar.gz",
			},
		},
	}

	err := Apply(rel, currentBin)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(currentBin)
	if err != nil {
		t.Fatalf("read updated binary: %v", err)
	}
	if string(data) != string(newContent) {
		t.Errorf("binary content = %q, want %q", string(data), string(newContent))
	}

	// Verify .old was cleaned up
	if _, err := os.Stat(currentBin + ".old"); err == nil {
		t.Error("expected .old file to be removed")
	}
}

func TestApply_NoAsset(t *testing.T) {
	rel := &Release{
		TagName: "v0.3.0",
		Assets:  []Asset{},
	}
	err := Apply(rel, "/tmp/fake")
	if err == nil {
		t.Error("expected error for missing asset")
	}
}

func TestExtractBinary(t *testing.T) {
	expected := []byte("binary-content")
	archiveData := createTestTarGz(t, "skillreg", expected)

	got, err := extractBinary(bytes.NewReader(archiveData))
	if err != nil {
		t.Fatalf("extractBinary failed: %v", err)
	}
	if string(got) != string(expected) {
		t.Errorf("got %q, want %q", string(got), string(expected))
	}
}

func TestExtractBinary_NotFound(t *testing.T) {
	archiveData := createTestTarGz(t, "wrong-name", []byte("data"))

	_, err := extractBinary(bytes.NewReader(archiveData))
	if err == nil {
		t.Error("expected error when binary not found in archive")
	}
}
