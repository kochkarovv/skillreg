# SkillRegistry — Agent Instructions

This document guides developers and AI agents working on SkillRegistry through the project architecture, conventions, and common development tasks.

## Project Overview

SkillRegistry is a terminal user interface (TUI) for managing custom skills across AI tools. The application:

- Discovers skills in local Git repositories
- Manages provider configurations (Claude, Codex, Gemini, etc.)
- Creates provider instances with custom aliases
- Installs and tracks skills via managed symlinks

## Architecture

SkillRegistry follows a clean, modular structure:

```
cmd/skillreg/
  main.go              # Entry point: database setup, TUI initialization

internal/
  config/
    config.go          # XDG paths (DataDir, DBPath) and constants

  db/
    db.go              # Database connection and lifecycle
    migrations.go      # Schema and default providers
    (crud files)       # CRUD operations for each model

  models/
    provider.go        # AI tools (Claude, Codex, etc.)
    instance.go        # Provider instances (aliased configurations)
    skill.go           # Discovered skills from repositories
    source.go          # Git repositories containing skills
    installation.go    # Active skill→instance relationships

  scanner/
    scanner.go         # Discover skills in source repositories

  git/
    git.go             # Git operations (clone, status, etc.)

  linker/
    linker.go          # Create/manage symlinks to installed skills

  tui/
    app.go             # BubbleTea app and state management
    menu_*.go          # Individual menu screens
    components/        # Reusable UI components (statusbar, confirm dialogs)
    styles.go          # Consistent styling
    keys.go            # Keyboard shortcuts
```

## Key Conventions

### 1. TUI Decoupling

- The TUI layer (`internal/tui/`) is **decoupled from business logic**
- All database operations are passed through the `tui.App` struct
- Menu files (`menu_*.go`) focus purely on UI state and rendering
- Database models are read-only in the TUI layer

### 2. BubbleTea v1

- Uses `charmbracelet/bubbletea` v1.3.10+
- Standard Elm model: `Model`, `Update`, `View`
- Status bar and confirm dialogs are reusable components
- Alt-screen mode is enabled for full-screen rendering

### 3. SQLite with Pure Go Driver

- Uses `modernc.org/sqlite` (pure Go, no C dependency)
- `CGO_ENABLED=0` for cross-platform builds
- Database path: `~/.local/share/skillreg/skillreg.db` (XDG-compliant)
- Migrations run on database open (idempotent)

### 4. Hardcoded Skills Directory Name

- The skills symlink directory is always named `skills`
- This is a convention, not configurable per provider
- Used in `internal/scanner/` and `internal/linker/`

### 5. Absolute Paths

- All file operations use **absolute paths only**
- Never use relative paths in symlinks or database records
- Use `filepath.Abs()` or expand `~` explicitly

## Testing

### Run All Tests

```bash
go test ./... -v
```

### Test Patterns

- Use `t.TempDir()` for isolated temporary directories in tests
- Each test creates its own temporary database
- No shared state between tests

### Example Test Structure

```go
func TestExample(t *testing.T) {
    // Setup
    tmpDir := t.TempDir()
    database, err := db.Open(filepath.Join(tmpDir, "test.db"))
    if err != nil {
        t.Fatalf("failed to open database: %v", err)
    }
    defer database.Close()

    // Exercise
    provider, err := models.GetProvider(database, 1)
    if err != nil {
        t.Fatalf("GetProvider failed: %v", err)
    }

    // Assert
    if provider.Name != "Claude" {
        t.Errorf("Expected Claude, got %s", provider.Name)
    }
}
```

## Common Development Tasks

### Adding a New Provider

1. **Update default providers** in `internal/db/migrations.go`:
   ```go
   {Name: "NewProvider", ConfigDirPrefix: ".newprovider"},
   ```

2. **Seed providers** runs automatically in `main.go` via `database.SeedProviders()`

3. **Add provider menu** if custom UI is needed (e.g., provider-specific configuration)

### Adding a Model Field

1. **Update the model struct** in `internal/models/<model>.go`
2. **Update the schema** in `internal/db/migrations.go` (add new columns)
3. **Update the scanner** in CRUD operations (`Scan` method)
4. **Update create/update functions** to handle the new field
5. **Write tests** covering the new field

### Modifying the TUI

1. **For new screens**: Create `internal/tui/menu_<name>.go` following the pattern in `menu_main.go`
2. **For new components**: Add to `internal/tui/components/` (e.g., `statusbar.go`, `confirm.go`)
3. **Keep styles consistent**: Use colors and spacing from `internal/tui/styles.go`
4. **Update the main app** in `internal/tui/app.go` to wire in new screens

### Debugging

- Print to stderr: `fmt.Fprintf(os.Stderr, "debug: %v\n", value)`
- Check database state: Query `skillreg.db` directly with `sqlite3`
- Inspect symlinks: `ls -la ~/.local/share/skillreg/` and in provider dirs

## Release Process

SkillRegistry uses GoReleaser for automated releases:

### Prerequisites

- Git tag format: `v1.0.0` (semantic versioning)
- `.goreleaser.yaml` configured for multi-platform builds

### Steps

1. **Ensure all tests pass**:
   ```bash
   go test ./... -v
   ```

2. **Tag the release**:
   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

3. **GitHub Actions Workflow** (`.github/workflows/release.yml`):
   - Runs tests
   - Builds binaries for Linux, macOS, Windows (amd64, arm64)
   - Generates checksums
   - Creates GitHub release with artifacts

### Build Output

- Binary name: `skillreg` (platform-specific)
- Archives: `skillreg_<OS>_<ARCH>.tar.gz` (or `.zip` for Windows)
- Checksums: `checksums.txt`

## Code Style

- **Package organization**: Each responsibility has its own package
- **Error handling**: Always wrap errors with context using `fmt.Errorf()`
- **Comments**: Exported functions and complex logic should have comment explanations
- **Naming**: Use clear, descriptive names; avoid abbreviations except for common ones (ID, DB, TUI, URL)

## Useful Commands

```bash
# Build locally
go build -o skillreg ./cmd/skillreg

# Run locally
go run ./cmd/skillreg

# Run tests with coverage
go test ./... -v -cover

# Format code
go fmt ./...

# Lint
golangci-lint run ./...

# Check for vet issues
go vet ./...
```

## Database Schema

Key tables:
- `providers` — AI tools (with `is_builtin` flag for defaults)
- `instances` — Provider instances with custom aliases
- `sources` — Git repositories containing skills
- `skills` — Discovered skills with metadata
- `installations` — Relationships between skills and instances

Query the database:
```bash
sqlite3 ~/.local/share/skillreg/skillreg.db
.schema
SELECT * FROM providers;
```

## References

- [BubbleTea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lipgloss (TUI styling)](https://github.com/charmbracelet/lipgloss)
- [SQLite Pure Go Driver](https://pkg.go.dev/modernc.org/sqlite)
- [GoReleaser](https://goreleaser.com/)
