# Auto-Update & Install Script Design

## Overview

Add seamless self-update capability to skillreg and a curl|sh install script. Target audience is developers comfortable with CLI tools on macOS/Linux.

## Components

### 1. Version Embedding

- Add `var Version = "dev"` in `cmd/skillreg/main.go`
- GoReleaser sets `-ldflags "-X main.Version={{.Version}}"` at build time
- `skillreg --version` prints `skillreg v0.3.0` and exits
- `Version == "dev"` skips update checks (local builds)

### 2. Update Check on Launch

**Package:** `internal/updater`

**Function:** `CheckLatest(currentVersion string) (*Release, error)`

- Calls `https://api.github.com/repos/kochkarovv/skillreg/releases/latest`
- Parses `tag_name` from JSON response
- Compares against embedded version using semver ordering (strip `v` prefix)
- Returns release info (tag, asset download URLs) if newer version exists
- Returns nil if already on latest

**Integration with TUI:**
- Spawned as a goroutine before TUI starts, result sent via channel
- 2-second timeout — if GitHub is slow or unreachable, silently skip
- Skip entirely if `Version == "dev"`

### 3. TUI Integration

**Banner:**
- `updateAvailableMsg` sent to BubbleTea app when newer version found
- `App` struct stores `availableUpdate *updater.Release`
- All views render a top banner when update is available:
  `Update available: v0.2.0 → v0.3.0 — press [u] to update`
- Rendered in highlight/accent style, above current view content

**`[u]` key:**
- Handled globally in `App.Update()` (like `ctrl+c`), not per-submenu
- Switches to `viewUpdate` showing download progress
- Update view shows: "Downloading v0.3.0..." → "Updated! Restart skillreg to use the new version." or error
- `esc` returns to previous view

### 4. Self-Replace Mechanism

**Function:** `Apply(release *Release) error`

**Steps:**
1. Match `runtime.GOOS` + `runtime.GOARCH` against release asset names (e.g. `skillreg_darwin_arm64.tar.gz`)
2. Download asset via HTTP GET, stream to temp file in same directory as running binary
3. Extract: `.tar.gz` on macOS/Linux, `.zip` on Windows
4. Replace running binary:
   - Get executable path via `os.Executable()` + `filepath.EvalSymlinks()`
   - Rename current binary to `skillreg.old`
   - Rename new binary to original path
   - Set permissions `0755`
   - Remove `skillreg.old`
5. On failure after rename: restore `skillreg.old` back to original path

**Platform notes:**
- Windows: running `.exe` can't be deleted but can be renamed; `.old` cleanup on next launch
- Permissions: if binary is in `/usr/local/bin` owned by root, fail with message suggesting `sudo skillreg update`

### 5. CLI Commands

**`--version` flag:**
- Handled in `main.go` before TUI startup via `os.Args` check
- Prints `skillreg v0.3.0` and exits

**`skillreg update` subcommand:**
- Checks for latest version, downloads and applies if newer
- Same `updater` package called outside TUI context
- If already on latest: `Already up to date (v0.3.0)`

### 6. Install Script

**File:** `install.sh` at repo root

**Steps:**
1. Detect OS (`uname -s` → darwin/linux) and arch (`uname -m` → x86_64/arm64, normalized to amd64/arm64)
2. Fetch latest release tag from GitHub API
3. Download `https://github.com/kochkarovv/skillreg/releases/download/{tag}/skillreg_{os}_{arch}.tar.gz`
4. Extract to temp directory
5. Move binary to `/usr/local/bin/skillreg` (fallback to `~/.local/bin` if no write access)
6. Verify with `skillreg --version`

**One-liner:** `curl -sSL https://raw.githubusercontent.com/kochkarovv/skillreg/main/install.sh | sh`

No Windows support in the script — Windows users download manually.

### 7. GoReleaser Changes

Add version ldflags to `.goreleaser.yaml` build section:
```yaml
ldflags:
  - -s -w -X main.Version={{.Version}}
```

## Files to Create/Modify

| File | Action |
|---|---|
| `internal/updater/updater.go` | Create — CheckLatest, Apply, Release struct |
| `internal/updater/updater_test.go` | Create — tests |
| `cmd/skillreg/main.go` | Modify — Version var, --version flag, update subcommand |
| `internal/tui/app.go` | Modify — update banner, [u] key, viewUpdate |
| `.goreleaser.yaml` | Modify — add version ldflags |
| `install.sh` | Create — curl\|sh installer |
| `README.md` | Modify — update install instructions |
