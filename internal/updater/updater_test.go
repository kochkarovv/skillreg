package updater

import (
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
