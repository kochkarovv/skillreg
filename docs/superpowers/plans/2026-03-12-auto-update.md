# Auto-Update & Install Script Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add self-update capability to skillreg so users get notified of new versions on launch and can update with one keypress, plus a curl|sh install script.

**Architecture:** A new `internal/updater` package handles version checking (GitHub Releases API) and binary self-replacement. The TUI shows a banner when updates are available and handles the `[u]` key globally. CLI gets `--version` and `update` subcommand before TUI startup.

**Tech Stack:** Go stdlib `net/http`, `archive/tar`, `compress/gzip`, `encoding/json`, `runtime`

---

## Chunk 1: Version Embedding & CLI Commands

### Task 1: GoReleaser ldflags and Version variable

**Files:**
- Modify: `.goreleaser.yaml:18-19`
- Modify: `cmd/skillreg/main.go`

- [ ] **Step 1: Add Version variable and CLI argument handling to main.go**

```go
// cmd/skillreg/main.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/config"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/tui"
)

// Version is set at build time via ldflags.
var Version = "dev"

func main() {
	// Handle --version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("skillreg %s\n", Version)
		os.Exit(0)
	}

	// Handle "update" subcommand — added in Task 4 after updater package exists
	// (placeholder comment for now)

	// Open (or create) the database.
	database, err := db.Open(config.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	// Seed built-in providers (idempotent).
	if err := database.SeedProviders(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: failed to seed providers: %v\n", err)
		os.Exit(1)
	}

	// Build and run the TUI.
	app := tui.NewApp(database)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Update GoReleaser ldflags**

In `.goreleaser.yaml`, change:
```yaml
    ldflags:
      - -s -w
```
To:
```yaml
    ldflags:
      - -s -w -X main.Version={{.Version}}
```

- [ ] **Step 3: Build and verify**

Run: `go build -o skillreg ./cmd/skillreg && ./skillreg --version`
Expected: `skillreg dev`

- [ ] **Step 4: Commit**

```bash
git add cmd/skillreg/main.go .goreleaser.yaml
git commit -m "feat: embed version at build time and add --version flag"
```

---

## Chunk 2: Updater Package — Version Check

### Task 2: Create updater package with CheckLatest

**Files:**
- Create: `internal/updater/updater.go`
- Create: `internal/updater/updater_test.go`

- [ ] **Step 1: Write the test for version comparison helper**

```go
// internal/updater/updater_test.go
package updater

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int // -1 = a<b, 0 = equal, 1 = a>b
	}{
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"0.2.0", "0.2.0", 0},
		{"1.0.0", "0.9.9", 1},
		{"0.10.0", "0.9.0", 1},
		{"1.2.3", "1.2.4", -1},
	}
	for _, tt := range tests {
		got := compareVersions(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updater/ -run TestCompareVersions -v`
Expected: FAIL — package doesn't exist yet

- [ ] **Step 3: Create updater.go with Release struct, compareVersions, and CheckLatest**

```go
// internal/updater/updater.go
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/updater/ -run TestCompareVersions -v`
Expected: PASS

- [ ] **Step 5: Write test for CheckLatest with dev version**

Add to `internal/updater/updater_test.go`:

```go
func TestCheckLatest_DevVersionSkips(t *testing.T) {
	rel, err := CheckLatest("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel != nil {
		t.Error("expected nil release for dev version")
	}
}
```

- [ ] **Step 6: Run test**

Run: `go test ./internal/updater/ -run TestCheckLatest_DevVersionSkips -v`
Expected: PASS

- [ ] **Step 7: Write test for AssetURL matching**

Add to `internal/updater/updater_test.go`:

```go
func TestRelease_AssetURL(t *testing.T) {
	rel := Release{
		TagName: "v0.3.0",
		Assets: []Asset{
			{Name: "skillreg_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64.tar.gz"},
			{Name: "skillreg_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64.tar.gz"},
			{Name: "skillreg_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64.zip"},
		},
	}
	url := rel.AssetURL()
	if url == "" {
		t.Fatal("expected a matching asset URL for current OS/arch")
	}
	// Just verify it returned one of the URLs (platform-dependent)
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
```

- [ ] **Step 8: Run all updater tests**

Run: `go test ./internal/updater/ -v`
Expected: All PASS

- [ ] **Step 9: Commit**

```bash
git add internal/updater/updater.go internal/updater/updater_test.go
git commit -m "feat: add updater package with version check via GitHub API"
```

---

## Chunk 3: Self-Replace Mechanism

### Task 3: Add Apply function for binary self-replacement

**Files:**
- Modify: `internal/updater/updater.go`
- Modify: `internal/updater/updater_test.go`

- [ ] **Step 1: Write test for Apply with a fake binary**

Add to `internal/updater/updater_test.go`:

```go
import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApply(t *testing.T) {
	// Create a fake "new" binary as a tar.gz
	newContent := []byte("#!/bin/sh\necho new-version\n")
	archiveData := createTestTarGz(t, "skillreg", newContent)

	// Serve the archive via httptest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(archiveData)
	}))
	defer srv.Close()

	// Create a fake "current" binary
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

	// Verify the binary was replaced
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
```

Note: add `"bytes"` to the imports.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/updater/ -run TestApply -v`
Expected: FAIL — `Apply` not defined

- [ ] **Step 3: Implement Apply in updater.go**

Add to `internal/updater/updater.go`:

```go
import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
)
```

```go
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

	// Download the asset
	resp, err := http.Get(assetURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	// Extract the binary from the tar.gz
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/updater/ -v`
Expected: All PASS

- [ ] **Step 5: Write test for Apply with missing asset**

Add to test file:

```go
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
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/updater/ -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/updater/updater.go internal/updater/updater_test.go
git commit -m "feat: add self-replace mechanism for binary updates"
```

---

## Chunk 4: TUI Integration

### Task 4: Wire update check into TUI and add [u] key handling

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/styles.go`
- Modify: `cmd/skillreg/main.go`

- [ ] **Step 1: Add update banner style to styles.go**

Add to `internal/tui/styles.go`:

```go
	updateBannerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#FF9900")).Padding(0, 2).Bold(true)
```

- [ ] **Step 2: Add update state and messages to app.go**

Add the `viewUpdate` constant, message types, and update fields to `App`:

```go
// Add to the view const block:
	viewUpdate

// Add new message types after the navigateMsg block:

// updateAvailableMsg is sent when a newer version is found.
type updateAvailableMsg struct {
	release *updater.Release
}

// updateDoneMsg is sent when the update download/apply finishes.
type updateDoneMsg struct {
	version string
	err     error
}
```

Add fields to `App` struct:

```go
	// Update
	availableUpdate *updater.Release
	updateStatus    string
	previousView    view
	version         string
```

- [ ] **Step 3: Update NewApp to accept version and start background check**

```go
func NewApp(d *db.Database, version string) App {
	return App{
		db:        d,
		current:   viewMain,
		main:      newMainMenu(d),
		skills:    newSkillsMenu(d),
		sources:   newSourcesMenu(d),
		providers: newProvidersMenu(d),
		tools:     newToolsMenu(d),
		version:   version,
	}
}
```

Update `Init()` to start background update check:

```go
func (a App) Init() tea.Cmd {
	return tea.Batch(a.main.Init(), a.checkForUpdate())
}

func (a App) checkForUpdate() tea.Cmd {
	return func() tea.Msg {
		rel, _ := updater.CheckLatest(a.version)
		if rel != nil {
			return updateAvailableMsg{release: rel}
		}
		return nil
	}
}
```

- [ ] **Step 4: Handle update messages and [u] key in App.Update()**

Add these cases to the `Update` method:

In the `switch msg := msg.(type)` block, add:

```go
	case updateAvailableMsg:
		a.availableUpdate = msg.release
		return a, nil

	case updateDoneMsg:
		if msg.err != nil {
			a.updateStatus = fmt.Sprintf("Update failed: %v", msg.err)
		} else {
			a.updateStatus = fmt.Sprintf("Updated to %s! Restart skillreg to use the new version.", msg.version)
		}
		return a, nil
```

In the `tea.KeyMsg` section, add before the ctrl+c check:

```go
		if msg.String() == "u" && a.availableUpdate != nil && a.current != viewUpdate {
			a.previousView = a.current
			a.current = viewUpdate
			a.updateStatus = fmt.Sprintf("Downloading %s...", a.availableUpdate.TagName)
			return a, a.applyUpdate()
		}
```

Add the `applyUpdate` method:

```go
func (a App) applyUpdate() tea.Cmd {
	rel := a.availableUpdate
	return func() tea.Msg {
		err := updater.Apply(rel, "")
		return updateDoneMsg{version: rel.TagName, err: err}
	}
}
```

- [ ] **Step 5: Add viewUpdate rendering in View()**

Add to the `View()` switch:

```go
	case viewUpdate:
		return a.viewUpdate()
```

Add the `viewUpdate` method and `updateBanner` helper:

```go
func (a App) viewUpdate() string {
	var sb strings.Builder
	sb.WriteString(titleStyle.Render("Update"))
	sb.WriteString("\n\n")
	sb.WriteString("  " + a.updateStatus)
	sb.WriteString("\n\n")
	sb.WriteString(subtleStyle.Render("esc back"))
	return sb.String()
}

func (a App) updateBanner() string {
	if a.availableUpdate == nil {
		return ""
	}
	return updateBannerStyle.Render(
		fmt.Sprintf("Update available: %s → %s — press [u] to update",
			a.version, a.availableUpdate.TagName)) + "\n"
}
```

- [ ] **Step 6: Prepend banner to all views**

Change `View()` to prepend the banner:

```go
func (a App) View() string {
	banner := a.updateBanner()
	var content string
	switch a.current {
	case viewUpdate:
		return a.viewUpdate()
	case viewSkills:
		content = a.skills.view()
	case viewSources:
		content = a.sources.view()
	case viewProviders:
		content = a.providers.view()
	case viewTools:
		content = a.tools.view()
	default:
		content = a.main.view()
	}
	return banner + content
}
```

- [ ] **Step 7: Handle esc in viewUpdate**

In the `tea.KeyMsg` section of `Update()`, add:

```go
		if a.current == viewUpdate && (msg.String() == "esc" || msg.String() == "q") {
			a.current = a.previousView
			return a, nil
		}
```

- [ ] **Step 8: Update main.go to pass Version to NewApp and add update subcommand**

```go
// cmd/skillreg/main.go — full final version
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/config"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/tui"
	"github.com/vladyslav/skillreg/internal/updater"
)

var Version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Printf("skillreg %s\n", Version)
			os.Exit(0)
		case "update":
			runUpdate()
			return
		}
	}

	database, err := db.Open(config.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.SeedProviders(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: failed to seed providers: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(database, Version)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "skillreg: %v\n", err)
		os.Exit(1)
	}
}

func runUpdate() {
	fmt.Printf("Checking for updates (current: %s)...\n", Version)
	rel, err := updater.CheckLatest(Version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
		os.Exit(1)
	}
	if rel == nil {
		fmt.Printf("Already up to date (%s)\n", Version)
		return
	}
	fmt.Printf("Downloading %s...\n", rel.TagName)
	if err := updater.Apply(rel, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Updated to %s! Restart skillreg to use the new version.\n", rel.TagName)
}
```

- [ ] **Step 9: Build and verify**

Run: `go build ./... && ./skillreg --version`
Expected: `skillreg dev`

- [ ] **Step 10: Commit**

```bash
git add internal/tui/app.go internal/tui/styles.go cmd/skillreg/main.go
git commit -m "feat: TUI update banner with [u] key and CLI update subcommand"
```

---

## Chunk 5: Install Script

### Task 5: Create install.sh

**Files:**
- Create: `install.sh`

- [ ] **Step 1: Write the install script**

```bash
#!/bin/sh
# Install script for skillreg
# Usage: curl -sSL https://raw.githubusercontent.com/kochkarovv/skillreg/main/install.sh | sh

set -e

REPO="kochkarovv/skillreg"

# Detect OS
OS="$(uname -s)"
case "$OS" in
    Linux*)  OS=linux;;
    Darwin*) OS=darwin;;
    *)       echo "Unsupported OS: $OS"; exit 1;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64)  ARCH=amd64;;
    aarch64) ARCH=arm64;;
    arm64)   ARCH=arm64;;
    *)       echo "Unsupported architecture: $ARCH"; exit 1;;
esac

echo "Detected platform: ${OS}/${ARCH}"

# Get latest release tag
echo "Fetching latest release..."
TAG=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release"
    exit 1
fi

echo "Latest version: ${TAG}"

# Download
URL="https://github.com/${REPO}/releases/download/${TAG}/skillreg_${OS}_${ARCH}.tar.gz"
TMPDIR=$(mktemp -d)
ARCHIVE="${TMPDIR}/skillreg.tar.gz"

echo "Downloading ${URL}..."
curl -sSL -o "$ARCHIVE" "$URL"

# Extract
echo "Extracting..."
tar -xzf "$ARCHIVE" -C "$TMPDIR"

# Install
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
    echo "No write access to /usr/local/bin, installing to ${INSTALL_DIR}"
fi

mv "${TMPDIR}/skillreg" "${INSTALL_DIR}/skillreg"
chmod +x "${INSTALL_DIR}/skillreg"

# Cleanup
rm -rf "$TMPDIR"

# Verify
if command -v skillreg >/dev/null 2>&1; then
    echo "Installed successfully: $(skillreg --version)"
else
    echo "Installed to ${INSTALL_DIR}/skillreg"
    echo "Make sure ${INSTALL_DIR} is in your PATH"
fi
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x install.sh`

- [ ] **Step 3: Commit**

```bash
git add install.sh
git commit -m "feat: add curl|sh install script"
```

---

## Chunk 6: README Update & Final Verification

### Task 6: Update README with new install instructions

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the Installation section in README.md**

Find the existing installation section and replace it with:

```markdown
## Installation

### Quick Install (macOS/Linux)

```sh
curl -sSL https://raw.githubusercontent.com/kochkarovv/skillreg/main/install.sh | sh
```

### Pre-built Binaries

Download the latest release from the [releases page](https://github.com/kochkarovv/skillreg/releases).

### Build from Source

```sh
git clone https://github.com/kochkarovv/skillreg.git
cd skillreg
go build -o skillreg ./cmd/skillreg
mv skillreg /usr/local/bin/
```

### Updating

skillreg checks for updates on launch. When a new version is available, press `[u]` to update. You can also update from the command line:

```sh
skillreg update
```
```

- [ ] **Step 2: Run full test suite**

Run: `go test ./... -count=1 -v`
Expected: All PASS

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: update install instructions with curl|sh and auto-update"
```

- [ ] **Step 4: Push and tag release**

```bash
git push
git tag v0.3.0
git push origin v0.3.0
```

Wait for GitHub Actions to complete and verify the release at https://github.com/kochkarovv/skillreg/releases/tag/v0.3.0
