# SkillReg Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go CLI tool with interactive TUI for managing agent skills across multiple AI coding assistant providers via symlinks and SQLite.

**Architecture:** BubbleTea TUI drives the interactive menu. Core logic (models, scanner, git, linker) is decoupled from the TUI layer and independently testable. SQLite stores all state (sources, providers, instances, skills, installations). Skills are symlinked from source repos to provider instance directories.

**Tech Stack:** Go 1.22+, BubbleTea/Bubbles/Lipgloss (Charm stack, v1 stable), SQLite via modernc.org/sqlite, GoReleaser for cross-platform releases.

**Spec:** `docs/superpowers/specs/2026-03-12-skillreg-design.md`

---

## File Structure

```
skillreg/
├── cmd/
│   └── skillreg/
│       └── main.go                  -- entry point: init DB, seed, launch TUI
├── internal/
│   ├── config/
│   │   └── config.go                -- XDG paths, constants
│   ├── db/
│   │   ├── db.go                    -- open/close DB, run migrations
│   │   └── migrations.go            -- SQL migration strings
│   ├── models/
│   │   ├── source.go                -- Source CRUD
│   │   ├── provider.go              -- Provider CRUD + seeding
│   │   ├── instance.go              -- Instance CRUD + home scan
│   │   ├── skill.go                 -- Skill CRUD
│   │   └── installation.go          -- Installation CRUD
│   ├── scanner/
│   │   └── scanner.go               -- scan repos for SKILL.md, parse description
│   ├── git/
│   │   └── git.go                   -- fetch, pull, status, stash, reset
│   ├── linker/
│   │   └── linker.go                -- symlink create/remove, backup, health check
│   └── tui/
│       ├── app.go                   -- root model, menu stack navigation
│       ├── styles.go                -- Lipgloss theme
│       ├── keys.go                  -- shared keybindings
│       ├── menu_main.go             -- main menu with update banner
│       ├── menu_skills.go           -- browse, install, uninstall skills
│       ├── menu_sources.go          -- add, manage, pull sources
│       ├── menu_providers.go        -- manage providers + instances
│       └── components/
│           ├── confirm.go           -- yes/no confirmation dialog
│           └── statusbar.go         -- bottom status bar
├── data/                            -- gitignored, dev/test only
├── .github/
│   └── workflows/
│       └── release.yml              -- GoReleaser build + GitHub Release
├── .goreleaser.yaml                 -- GoReleaser config
├── go.mod
├── go.sum
├── .gitignore
├── README.md
└── AGENTS.md
```

---

## Chunk 1: Project Scaffolding, Config, and Database Layer

### Task 1: Initialize Go module and dependencies

**Files:**
- Create: `go.mod`
- Create: `go.sum`
- Create: `.gitignore`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/vladyslav/Projects/SkillRegistry
go mod init github.com/vladyslav/skillreg
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get modernc.org/sqlite@latest
```

- [ ] **Step 3: Create .gitignore**

```gitignore
# Binaries
skillreg
*.exe

# Data
data/
*.db

# OS
.DS_Store

# IDE
.idea/
.vscode/

# Go
vendor/
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum .gitignore
git commit -m "feat: initialize Go module with dependencies"
```

---

### Task 2: Config — XDG paths and constants

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/config/config_test.go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDataDir_Default(t *testing.T) {
	os.Unsetenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".local", "share", "skillreg")
	got := DataDir()
	if got != expected {
		t.Errorf("DataDir() = %q, want %q", got, expected)
	}
}

func TestDataDir_CustomXDG(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/test-xdg")
	expected := filepath.Join("/tmp/test-xdg", "skillreg")
	got := DataDir()
	if got != expected {
		t.Errorf("DataDir() = %q, want %q", got, expected)
	}
}

func TestDBPath(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/test-xdg")
	expected := filepath.Join("/tmp/test-xdg", "skillreg", "skillreg.db")
	got := DBPath()
	if got != expected {
		t.Errorf("DBPath() = %q, want %q", got, expected)
	}
}

func TestSkillsDirName(t *testing.T) {
	if SkillsDirName != "skills" {
		t.Errorf("SkillsDirName = %q, want %q", SkillsDirName, "skills")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/vladyslav/Projects/SkillRegistry
go test ./internal/config/ -v
```
Expected: FAIL — package doesn't exist yet.

- [ ] **Step 3: Write implementation**

```go
// internal/config/config.go
package config

import (
	"os"
	"path/filepath"
)

const SkillsDirName = "skills"

var ExcludedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".venv":        true,
}

func DataDir() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "skillreg")
}

func DBPath() string {
	return filepath.Join(DataDir(), "skillreg.db")
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/config/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat: add config package with XDG paths and constants"
```

---

### Task 3: Database — connection, migrations, seeding

**Files:**
- Create: `internal/db/migrations.go`
- Create: `internal/db/db.go`
- Test: `internal/db/db_test.go`

- [ ] **Step 1: Write migrations.go**

```go
// internal/db/migrations.go
package db

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS sources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		remote_url TEXT NOT NULL DEFAULT '',
		auto_update BOOLEAN NOT NULL DEFAULT 0,
		last_checked_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE IF NOT EXISTS providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		config_dir_prefix TEXT NOT NULL,
		is_builtin BOOLEAN NOT NULL DEFAULT 0
	);`,
	`CREATE TABLE IF NOT EXISTS instances (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id INTEGER NOT NULL REFERENCES providers(id),
		name TEXT NOT NULL UNIQUE,
		global_skills_path TEXT NOT NULL UNIQUE,
		is_default BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`,
	`CREATE TABLE IF NOT EXISTS skills (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id INTEGER NOT NULL REFERENCES sources(id),
		name TEXT NOT NULL,
		original_path TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		discovered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source_id, original_path)
	);`,
	`CREATE TABLE IF NOT EXISTS installations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id INTEGER NOT NULL REFERENCES skills(id),
		instance_id INTEGER NOT NULL REFERENCES instances(id),
		symlink_path TEXT NOT NULL,
		installed_name TEXT NOT NULL,
		installed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'active',
		UNIQUE(skill_id, instance_id)
	);`,
}

type DefaultProvider struct {
	Name            string
	ConfigDirPrefix string
}

var DefaultProviders = []DefaultProvider{
	{Name: "Claude", ConfigDirPrefix: ".claude"},
	{Name: "Codex", ConfigDirPrefix: ".agents"},
	{Name: "Gemini", ConfigDirPrefix: ".gemini"},
	{Name: "Cursor", ConfigDirPrefix: ".cursor"},
	{Name: "VSCode / Copilot", ConfigDirPrefix: ".github"},
	{Name: "Antigravity", ConfigDirPrefix: ".agents"},
}
```

- [ ] **Step 2: Write the test for db.go**

```go
// internal/db/db_test.go
package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesDBFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer d.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("DB file was not created")
	}
}

func TestOpen_RunsMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer d.Close()

	tables := []string{"sources", "providers", "instances", "skills", "installations"}
	for _, table := range tables {
		var name string
		err := d.DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found: %v", table, err)
		}
	}
}

func TestSeedProviders(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer d.Close()

	err = d.SeedProviders()
	if err != nil {
		t.Fatalf("SeedProviders() error: %v", err)
	}

	var count int
	d.DB.QueryRow("SELECT COUNT(*) FROM providers").Scan(&count)
	if count != len(DefaultProviders) {
		t.Errorf("expected %d providers, got %d", len(DefaultProviders), count)
	}

	// Calling again should not duplicate
	err = d.SeedProviders()
	if err != nil {
		t.Fatalf("SeedProviders() second call error: %v", err)
	}

	d.DB.QueryRow("SELECT COUNT(*) FROM providers").Scan(&count)
	if count != len(DefaultProviders) {
		t.Errorf("expected %d providers after re-seed, got %d", len(DefaultProviders), count)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/db/ -v
```
Expected: FAIL — db.go doesn't exist yet.

- [ ] **Step 4: Write db.go implementation**

```go
// internal/db/db.go
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Database struct {
	DB *sql.DB
}

func Open(path string) (*Database, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", path+"?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	d := &Database{DB: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return d, nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

func (d *Database) migrate() error {
	for i, m := range migrations {
		if _, err := d.DB.Exec(m); err != nil {
			return fmt.Errorf("migration %d: %w", i, err)
		}
	}
	return nil
}

func (d *Database) SeedProviders() error {
	stmt, err := d.DB.Prepare(
		"INSERT OR IGNORE INTO providers (name, config_dir_prefix, is_builtin) VALUES (?, ?, 1)",
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range DefaultProviders {
		if _, err := stmt.Exec(p.Name, p.ConfigDirPrefix); err != nil {
			return fmt.Errorf("seed provider %q: %w", p.Name, err)
		}
	}
	return nil
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
go test ./internal/db/ -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/db/
git commit -m "feat: add database layer with migrations and provider seeding"
```

---

## Chunk 2: Core Models (CRUD)

### Task 4: Source model

**Files:**
- Create: `internal/models/source.go`
- Test: `internal/models/source_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/models/source_test.go
package models

import (
	"path/filepath"
	"testing"

	"github.com/vladyslav/skillreg/internal/db"
)

func newTestDB(t *testing.T) *db.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSourceCreate(t *testing.T) {
	d := newTestDB(t)

	s, err := CreateSource(d, "superpowers", "/path/to/repo", "https://github.com/test/superpowers")
	if err != nil {
		t.Fatalf("CreateSource() error: %v", err)
	}
	if s.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if s.Name != "superpowers" {
		t.Errorf("Name = %q, want %q", s.Name, "superpowers")
	}
}

func TestSourceList(t *testing.T) {
	d := newTestDB(t)

	CreateSource(d, "a", "/a", "")
	CreateSource(d, "b", "/b", "")

	sources, err := ListSources(d)
	if err != nil {
		t.Fatalf("ListSources() error: %v", err)
	}
	if len(sources) != 2 {
		t.Errorf("got %d sources, want 2", len(sources))
	}
}

func TestSourceDelete(t *testing.T) {
	d := newTestDB(t)

	s, _ := CreateSource(d, "a", "/a", "")
	err := DeleteSource(d, s.ID)
	if err != nil {
		t.Fatalf("DeleteSource() error: %v", err)
	}

	sources, _ := ListSources(d)
	if len(sources) != 0 {
		t.Errorf("got %d sources, want 0", len(sources))
	}
}

func TestSourceUpdateAutoUpdate(t *testing.T) {
	d := newTestDB(t)

	s, _ := CreateSource(d, "a", "/a", "")
	if s.AutoUpdate {
		t.Error("expected AutoUpdate to be false initially")
	}

	err := SetSourceAutoUpdate(d, s.ID, true)
	if err != nil {
		t.Fatalf("SetSourceAutoUpdate() error: %v", err)
	}

	s2, _ := GetSource(d, s.ID)
	if !s2.AutoUpdate {
		t.Error("expected AutoUpdate to be true after update")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/models/ -v -run TestSource
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/models/source.go
package models

import (
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

type Source struct {
	ID            int64
	Name          string
	Path          string
	RemoteURL     string
	AutoUpdate    bool
	LastCheckedAt *time.Time
	CreatedAt     time.Time
}

func CreateSource(d *db.Database, name, path, remoteURL string) (*Source, error) {
	res, err := d.DB.Exec(
		"INSERT INTO sources (name, path, remote_url) VALUES (?, ?, ?)",
		name, path, remoteURL,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetSource(d, id)
}

func GetSource(d *db.Database, id int64) (*Source, error) {
	s := &Source{}
	err := d.DB.QueryRow(
		"SELECT id, name, path, remote_url, auto_update, last_checked_at, created_at FROM sources WHERE id = ?", id,
	).Scan(&s.ID, &s.Name, &s.Path, &s.RemoteURL, &s.AutoUpdate, &s.LastCheckedAt, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ListSources(d *db.Database) ([]Source, error) {
	rows, err := d.DB.Query(
		"SELECT id, name, path, remote_url, auto_update, last_checked_at, created_at FROM sources ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var s Source
		if err := rows.Scan(&s.ID, &s.Name, &s.Path, &s.RemoteURL, &s.AutoUpdate, &s.LastCheckedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

func DeleteSource(d *db.Database, id int64) error {
	_, err := d.DB.Exec("DELETE FROM sources WHERE id = ?", id)
	return err
}

func SetSourceAutoUpdate(d *db.Database, id int64, autoUpdate bool) error {
	_, err := d.DB.Exec("UPDATE sources SET auto_update = ? WHERE id = ?", autoUpdate, id)
	return err
}

func UpdateSourceLastChecked(d *db.Database, id int64) error {
	_, err := d.DB.Exec("UPDATE sources SET last_checked_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/models/ -v -run TestSource
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/source.go internal/models/source_test.go
git commit -m "feat: add Source model with CRUD operations"
```

---

### Task 5: Provider model

**Files:**
- Create: `internal/models/provider.go`
- Test: `internal/models/provider_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/models/provider_test.go
package models

import (
	"testing"
)

func TestListProviders(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, err := ListProviders(d)
	if err != nil {
		t.Fatalf("ListProviders() error: %v", err)
	}
	if len(providers) != 6 {
		t.Errorf("got %d providers, want 6", len(providers))
	}
}

func TestGetProvider(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, _ := ListProviders(d)
	p, err := GetProvider(d, providers[0].ID)
	if err != nil {
		t.Fatalf("GetProvider() error: %v", err)
	}
	if p.Name != providers[0].Name {
		t.Errorf("Name = %q, want %q", p.Name, providers[0].Name)
	}
}

func TestCreateProvider(t *testing.T) {
	d := newTestDB(t)

	p, err := CreateProvider(d, "CustomAgent", ".custom")
	if err != nil {
		t.Fatalf("CreateProvider() error: %v", err)
	}
	if p.IsBuiltin {
		t.Error("expected IsBuiltin to be false for user-created provider")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/models/ -v -run TestProvider -run TestGetProvider -run TestCreateProvider
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/models/provider.go
package models

import (
	"github.com/vladyslav/skillreg/internal/db"
)

type Provider struct {
	ID              int64
	Name            string
	ConfigDirPrefix string
	IsBuiltin       bool
}

func ListProviders(d *db.Database) ([]Provider, error) {
	rows, err := d.DB.Query(
		"SELECT id, name, config_dir_prefix, is_builtin FROM providers ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []Provider
	for rows.Next() {
		var p Provider
		if err := rows.Scan(&p.ID, &p.Name, &p.ConfigDirPrefix, &p.IsBuiltin); err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

func GetProvider(d *db.Database, id int64) (*Provider, error) {
	p := &Provider{}
	err := d.DB.QueryRow(
		"SELECT id, name, config_dir_prefix, is_builtin FROM providers WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.ConfigDirPrefix, &p.IsBuiltin)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func CreateProvider(d *db.Database, name, configDirPrefix string) (*Provider, error) {
	res, err := d.DB.Exec(
		"INSERT INTO providers (name, config_dir_prefix, is_builtin) VALUES (?, ?, 0)",
		name, configDirPrefix,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetProvider(d, id)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/models/ -v -run "TestProvider|TestGetProvider|TestCreateProvider"
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/provider.go internal/models/provider_test.go
git commit -m "feat: add Provider model with CRUD operations"
```

---

### Task 6: Instance model

**Files:**
- Create: `internal/models/instance.go`
- Test: `internal/models/instance_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/models/instance_test.go
package models

import (
	"testing"
)

func TestInstanceCreate(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, _ := ListProviders(d)
	inst, err := CreateInstance(d, providers[0].ID, "claude-personal", "/home/user/.claude-personal/skills", true)
	if err != nil {
		t.Fatalf("CreateInstance() error: %v", err)
	}
	if inst.Name != "claude-personal" {
		t.Errorf("Name = %q, want %q", inst.Name, "claude-personal")
	}
	if !inst.IsDefault {
		t.Error("expected IsDefault to be true")
	}
}

func TestInstanceUniqueSkillsPath(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, _ := ListProviders(d)
	CreateInstance(d, providers[0].ID, "inst-a", "/same/path/skills", false)
	_, err := CreateInstance(d, providers[0].ID, "inst-b", "/same/path/skills", false)
	if err == nil {
		t.Error("expected error for duplicate global_skills_path")
	}
}

func TestInstanceListByProvider(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, _ := ListProviders(d)
	CreateInstance(d, providers[0].ID, "a", "/a/skills", false)
	CreateInstance(d, providers[0].ID, "b", "/b/skills", false)

	instances, err := ListInstancesByProvider(d, providers[0].ID)
	if err != nil {
		t.Fatalf("ListInstancesByProvider() error: %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("got %d instances, want 2", len(instances))
	}
}

func TestInstanceListAll(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, _ := ListProviders(d)
	CreateInstance(d, providers[0].ID, "a", "/a/skills", false)
	CreateInstance(d, providers[1].ID, "b", "/b/skills", false)

	instances, err := ListAllInstances(d)
	if err != nil {
		t.Fatalf("ListAllInstances() error: %v", err)
	}
	if len(instances) != 2 {
		t.Errorf("got %d instances, want 2", len(instances))
	}
}

func TestInstanceDelete(t *testing.T) {
	d := newTestDB(t)
	d.SeedProviders()

	providers, _ := ListProviders(d)
	inst, _ := CreateInstance(d, providers[0].ID, "a", "/a/skills", false)
	err := DeleteInstance(d, inst.ID)
	if err != nil {
		t.Fatalf("DeleteInstance() error: %v", err)
	}

	all, _ := ListAllInstances(d)
	if len(all) != 0 {
		t.Errorf("got %d instances, want 0", len(all))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/models/ -v -run TestInstance
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/models/instance.go
package models

import (
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

type Instance struct {
	ID               int64
	ProviderID       int64
	Name             string
	GlobalSkillsPath string
	IsDefault        bool
	CreatedAt        time.Time
}

func CreateInstance(d *db.Database, providerID int64, name, globalSkillsPath string, isDefault bool) (*Instance, error) {
	res, err := d.DB.Exec(
		"INSERT INTO instances (provider_id, name, global_skills_path, is_default) VALUES (?, ?, ?, ?)",
		providerID, name, globalSkillsPath, isDefault,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetInstance(d, id)
}

func GetInstance(d *db.Database, id int64) (*Instance, error) {
	inst := &Instance{}
	err := d.DB.QueryRow(
		"SELECT id, provider_id, name, global_skills_path, is_default, created_at FROM instances WHERE id = ?", id,
	).Scan(&inst.ID, &inst.ProviderID, &inst.Name, &inst.GlobalSkillsPath, &inst.IsDefault, &inst.CreatedAt)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func ListInstancesByProvider(d *db.Database, providerID int64) ([]Instance, error) {
	rows, err := d.DB.Query(
		"SELECT id, provider_id, name, global_skills_path, is_default, created_at FROM instances WHERE provider_id = ? ORDER BY name",
		providerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var inst Instance
		if err := rows.Scan(&inst.ID, &inst.ProviderID, &inst.Name, &inst.GlobalSkillsPath, &inst.IsDefault, &inst.CreatedAt); err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

func ListAllInstances(d *db.Database) ([]Instance, error) {
	rows, err := d.DB.Query(
		"SELECT id, provider_id, name, global_skills_path, is_default, created_at FROM instances ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []Instance
	for rows.Next() {
		var inst Instance
		if err := rows.Scan(&inst.ID, &inst.ProviderID, &inst.Name, &inst.GlobalSkillsPath, &inst.IsDefault, &inst.CreatedAt); err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

func DeleteInstance(d *db.Database, id int64) error {
	_, err := d.DB.Exec("DELETE FROM instances WHERE id = ?", id)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/models/ -v -run TestInstance
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/instance.go internal/models/instance_test.go
git commit -m "feat: add Instance model with CRUD operations"
```

---

### Task 7: Skill model

**Files:**
- Create: `internal/models/skill.go`
- Test: `internal/models/skill_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/models/skill_test.go
package models

import (
	"testing"
)

func TestSkillCreate(t *testing.T) {
	d := newTestDB(t)
	s, _ := CreateSource(d, "src", "/src", "")

	skill, err := CreateSkill(d, s.ID, "brainstorming", "/src/skills/brainstorming", "A brainstorming skill")
	if err != nil {
		t.Fatalf("CreateSkill() error: %v", err)
	}
	if skill.Name != "brainstorming" {
		t.Errorf("Name = %q, want %q", skill.Name, "brainstorming")
	}
}

func TestSkillUniqueBySourceAndPath(t *testing.T) {
	d := newTestDB(t)
	s, _ := CreateSource(d, "src", "/src", "")

	CreateSkill(d, s.ID, "brainstorming", "/src/skills/brainstorming", "")
	_, err := CreateSkill(d, s.ID, "brainstorming", "/src/skills/brainstorming", "")
	if err == nil {
		t.Error("expected error for duplicate source_id + original_path")
	}
}

func TestSkillListBySource(t *testing.T) {
	d := newTestDB(t)
	s, _ := CreateSource(d, "src", "/src", "")

	CreateSkill(d, s.ID, "a", "/src/a", "")
	CreateSkill(d, s.ID, "b", "/src/b", "")

	skills, err := ListSkillsBySource(d, s.ID)
	if err != nil {
		t.Fatalf("ListSkillsBySource() error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("got %d skills, want 2", len(skills))
	}
}

func TestSkillListAll(t *testing.T) {
	d := newTestDB(t)
	s1, _ := CreateSource(d, "src1", "/src1", "")
	s2, _ := CreateSource(d, "src2", "/src2", "")

	CreateSkill(d, s1.ID, "a", "/src1/a", "")
	CreateSkill(d, s2.ID, "b", "/src2/b", "")

	skills, err := ListAllSkills(d)
	if err != nil {
		t.Fatalf("ListAllSkills() error: %v", err)
	}
	if len(skills) != 2 {
		t.Errorf("got %d skills, want 2", len(skills))
	}
}

func TestSkillDeleteBySource(t *testing.T) {
	d := newTestDB(t)
	s, _ := CreateSource(d, "src", "/src", "")

	CreateSkill(d, s.ID, "a", "/src/a", "")
	CreateSkill(d, s.ID, "b", "/src/b", "")

	err := DeleteSkillsBySource(d, s.ID)
	if err != nil {
		t.Fatalf("DeleteSkillsBySource() error: %v", err)
	}

	skills, _ := ListSkillsBySource(d, s.ID)
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/models/ -v -run TestSkill
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/models/skill.go
package models

import (
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

type Skill struct {
	ID           int64
	SourceID     int64
	Name         string
	OriginalPath string
	Description  string
	DiscoveredAt time.Time
}

func CreateSkill(d *db.Database, sourceID int64, name, originalPath, description string) (*Skill, error) {
	res, err := d.DB.Exec(
		"INSERT INTO skills (source_id, name, original_path, description) VALUES (?, ?, ?, ?)",
		sourceID, name, originalPath, description,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetSkill(d, id)
}

func GetSkill(d *db.Database, id int64) (*Skill, error) {
	s := &Skill{}
	err := d.DB.QueryRow(
		"SELECT id, source_id, name, original_path, description, discovered_at FROM skills WHERE id = ?", id,
	).Scan(&s.ID, &s.SourceID, &s.Name, &s.OriginalPath, &s.Description, &s.DiscoveredAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func ListSkillsBySource(d *db.Database, sourceID int64) ([]Skill, error) {
	rows, err := d.DB.Query(
		"SELECT id, source_id, name, original_path, description, discovered_at FROM skills WHERE source_id = ? ORDER BY name",
		sourceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.SourceID, &s.Name, &s.OriginalPath, &s.Description, &s.DiscoveredAt); err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

func ListAllSkills(d *db.Database) ([]Skill, error) {
	rows, err := d.DB.Query(
		"SELECT id, source_id, name, original_path, description, discovered_at FROM skills ORDER BY name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var skills []Skill
	for rows.Next() {
		var s Skill
		if err := rows.Scan(&s.ID, &s.SourceID, &s.Name, &s.OriginalPath, &s.Description, &s.DiscoveredAt); err != nil {
			return nil, err
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

func DeleteSkillsBySource(d *db.Database, sourceID int64) error {
	_, err := d.DB.Exec("DELETE FROM skills WHERE source_id = ?", sourceID)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/models/ -v -run TestSkill
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/skill.go internal/models/skill_test.go
git commit -m "feat: add Skill model with CRUD operations"
```

---

### Task 8: Installation model

**Files:**
- Create: `internal/models/installation.go`
- Test: `internal/models/installation_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/models/installation_test.go
package models

import (
	"testing"
)

func setupInstallationTest(t *testing.T) (*db.Database, *Skill, *Instance) {
	t.Helper()
	d := newTestDB(t)
	d.SeedProviders()

	src, _ := CreateSource(d, "src", "/src", "")
	skill, _ := CreateSkill(d, src.ID, "test-skill", "/src/test-skill", "")

	providers, _ := ListProviders(d)
	inst, _ := CreateInstance(d, providers[0].ID, "claude-test", "/home/.claude-test/skills", false)

	return d, skill, inst
}

func TestInstallationCreate(t *testing.T) {
	d, skill, inst := setupInstallationTest(t)

	installation, err := CreateInstallation(d, skill.ID, inst.ID, "/home/.claude-test/skills/test-skill", "test-skill")
	if err != nil {
		t.Fatalf("CreateInstallation() error: %v", err)
	}
	if installation.Status != "active" {
		t.Errorf("Status = %q, want %q", installation.Status, "active")
	}
}

func TestInstallationUniqueSkillInstance(t *testing.T) {
	d, skill, inst := setupInstallationTest(t)

	CreateInstallation(d, skill.ID, inst.ID, "/path/a", "test-skill")
	_, err := CreateInstallation(d, skill.ID, inst.ID, "/path/b", "test-skill-2")
	if err == nil {
		t.Error("expected error for duplicate skill_id + instance_id")
	}
}

func TestInstallationListByInstance(t *testing.T) {
	d, skill, inst := setupInstallationTest(t)
	src, _ := GetSource(d, skill.SourceID)
	_ = src

	CreateInstallation(d, skill.ID, inst.ID, "/path/a", "test-skill")

	installations, err := ListInstallationsByInstance(d, inst.ID)
	if err != nil {
		t.Fatalf("ListInstallationsByInstance() error: %v", err)
	}
	if len(installations) != 1 {
		t.Errorf("got %d installations, want 1", len(installations))
	}
}

func TestInstallationListBySkill(t *testing.T) {
	d, skill, inst := setupInstallationTest(t)

	CreateInstallation(d, skill.ID, inst.ID, "/path/a", "test-skill")

	installations, err := ListInstallationsBySkill(d, skill.ID)
	if err != nil {
		t.Fatalf("ListInstallationsBySkill() error: %v", err)
	}
	if len(installations) != 1 {
		t.Errorf("got %d installations, want 1", len(installations))
	}
}

func TestInstallationUpdateStatus(t *testing.T) {
	d, skill, inst := setupInstallationTest(t)

	installation, _ := CreateInstallation(d, skill.ID, inst.ID, "/path/a", "test-skill")
	err := UpdateInstallationStatus(d, installation.ID, "broken")
	if err != nil {
		t.Fatalf("UpdateInstallationStatus() error: %v", err)
	}

	updated, _ := GetInstallation(d, installation.ID)
	if updated.Status != "broken" {
		t.Errorf("Status = %q, want %q", updated.Status, "broken")
	}
}

func TestInstallationDelete(t *testing.T) {
	d, skill, inst := setupInstallationTest(t)

	installation, _ := CreateInstallation(d, skill.ID, inst.ID, "/path/a", "test-skill")
	err := DeleteInstallation(d, installation.ID)
	if err != nil {
		t.Fatalf("DeleteInstallation() error: %v", err)
	}

	all, _ := ListInstallationsByInstance(d, inst.ID)
	if len(all) != 0 {
		t.Errorf("got %d installations, want 0", len(all))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/models/ -v -run TestInstallation
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/models/installation.go
package models

import (
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

type Installation struct {
	ID            int64
	SkillID       int64
	InstanceID    int64
	SymlinkPath   string
	InstalledName string
	InstalledAt   time.Time
	Status        string
}

func CreateInstallation(d *db.Database, skillID, instanceID int64, symlinkPath, installedName string) (*Installation, error) {
	res, err := d.DB.Exec(
		"INSERT INTO installations (skill_id, instance_id, symlink_path, installed_name) VALUES (?, ?, ?, ?)",
		skillID, instanceID, symlinkPath, installedName,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetInstallation(d, id)
}

func GetInstallation(d *db.Database, id int64) (*Installation, error) {
	inst := &Installation{}
	err := d.DB.QueryRow(
		"SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations WHERE id = ?", id,
	).Scan(&inst.ID, &inst.SkillID, &inst.InstanceID, &inst.SymlinkPath, &inst.InstalledName, &inst.InstalledAt, &inst.Status)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func ListInstallationsByInstance(d *db.Database, instanceID int64) ([]Installation, error) {
	rows, err := d.DB.Query(
		"SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations WHERE instance_id = ? ORDER BY installed_name",
		instanceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []Installation
	for rows.Next() {
		var inst Installation
		if err := rows.Scan(&inst.ID, &inst.SkillID, &inst.InstanceID, &inst.SymlinkPath, &inst.InstalledName, &inst.InstalledAt, &inst.Status); err != nil {
			return nil, err
		}
		installations = append(installations, inst)
	}
	return installations, rows.Err()
}

func ListInstallationsBySkill(d *db.Database, skillID int64) ([]Installation, error) {
	rows, err := d.DB.Query(
		"SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations WHERE skill_id = ? ORDER BY installed_name",
		skillID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []Installation
	for rows.Next() {
		var inst Installation
		if err := rows.Scan(&inst.ID, &inst.SkillID, &inst.InstanceID, &inst.SymlinkPath, &inst.InstalledName, &inst.InstalledAt, &inst.Status); err != nil {
			return nil, err
		}
		installations = append(installations, inst)
	}
	return installations, rows.Err()
}

func ListAllInstallations(d *db.Database) ([]Installation, error) {
	rows, err := d.DB.Query(
		"SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations ORDER BY installed_name",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var installations []Installation
	for rows.Next() {
		var inst Installation
		if err := rows.Scan(&inst.ID, &inst.SkillID, &inst.InstanceID, &inst.SymlinkPath, &inst.InstalledName, &inst.InstalledAt, &inst.Status); err != nil {
			return nil, err
		}
		installations = append(installations, inst)
	}
	return installations, rows.Err()
}

func UpdateInstallationStatus(d *db.Database, id int64, status string) error {
	_, err := d.DB.Exec("UPDATE installations SET status = ? WHERE id = ?", status, id)
	return err
}

func DeleteInstallation(d *db.Database, id int64) error {
	_, err := d.DB.Exec("DELETE FROM installations WHERE id = ?", id)
	return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/models/ -v -run TestInstallation
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/models/installation.go internal/models/installation_test.go
git commit -m "feat: add Installation model with CRUD operations"
```

---

## Chunk 3: Scanner, Git, and Linker

### Task 9: Scanner — discover skills in source repos

**Files:**
- Create: `internal/scanner/scanner.go`
- Test: `internal/scanner/scanner_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/scanner/scanner_test.go
package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func createSkillDir(t *testing.T, base, name, content string) {
	t.Helper()
	dir := filepath.Join(base, name)
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644)
}

func TestScanRepo_FindsTopLevelSkills(t *testing.T) {
	root := t.TempDir()
	createSkillDir(t, root, "my-skill", "---\ndescription: A cool skill\n---\n# My Skill")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatalf("ScanRepo() error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "my-skill")
	}
	if skills[0].Description != "A cool skill" {
		t.Errorf("Description = %q, want %q", skills[0].Description, "A cool skill")
	}
}

func TestScanRepo_FindsNestedSkills(t *testing.T) {
	root := t.TempDir()
	createSkillDir(t, filepath.Join(root, "skills"), "brainstorming", "# Brainstorming\nHelp brainstorm ideas")
	createSkillDir(t, filepath.Join(root, ".claude", "skills"), "debug", "Debug skill")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatalf("ScanRepo() error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}
}

func TestScanRepo_ExcludesDotGit(t *testing.T) {
	root := t.TempDir()
	createSkillDir(t, filepath.Join(root, ".git", "hooks"), "pre-commit", "not a skill")
	createSkillDir(t, root, "real-skill", "Real skill")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatalf("ScanRepo() error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "real-skill" {
		t.Errorf("Name = %q, want %q", skills[0].Name, "real-skill")
	}
}

func TestScanRepo_ExcludesNodeModules(t *testing.T) {
	root := t.TempDir()
	createSkillDir(t, filepath.Join(root, "node_modules", "pkg"), "fake", "fake")
	createSkillDir(t, root, "real", "real")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatalf("ScanRepo() error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
}

func TestParseDescription_Frontmatter(t *testing.T) {
	content := "---\nname: test\ndescription: My description here\n---\n# Heading\nBody"
	desc := ParseDescription(content)
	if desc != "My description here" {
		t.Errorf("ParseDescription() = %q, want %q", desc, "My description here")
	}
}

func TestParseDescription_NoFrontmatter(t *testing.T) {
	content := "# My Skill\nThis is the first real line.\nAnother line."
	desc := ParseDescription(content)
	if desc != "This is the first real line." {
		t.Errorf("ParseDescription() = %q, want %q", desc, "This is the first real line.")
	}
}

func TestParseDescription_Empty(t *testing.T) {
	desc := ParseDescription("")
	if desc != "" {
		t.Errorf("ParseDescription() = %q, want empty string", desc)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/scanner/ -v
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/scanner/scanner.go
package scanner

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/vladyslav/skillreg/internal/config"
)

type DiscoveredSkill struct {
	Name        string
	Path        string
	Description string
}

func ScanRepo(repoPath string) ([]DiscoveredSkill, error) {
	var skills []DiscoveredSkill

	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible dirs
		}

		if d.IsDir() {
			name := d.Name()
			if config.ExcludedDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Name() != "SKILL.md" {
			return nil
		}

		skillDir := filepath.Dir(path)
		skillName := filepath.Base(skillDir)

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}

		skills = append(skills, DiscoveredSkill{
			Name:        skillName,
			Path:        skillDir,
			Description: ParseDescription(string(content)),
		})

		return nil
	})

	return skills, err
}

func ParseDescription(content string) string {
	if content == "" {
		return ""
	}

	// Try YAML frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "description:") {
					desc := strings.TrimPrefix(line, "description:")
					desc = strings.TrimSpace(desc)
					desc = strings.Trim(desc, "\"'")
					return desc
				}
			}
		}
	}

	// Fallback: first non-empty, non-heading line
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			continue
		}
		return line
	}

	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/scanner/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/scanner/
git commit -m "feat: add scanner for discovering skills in source repos"
```

---

### Task 10: Git — fetch, pull, status operations

**Files:**
- Create: `internal/git/git.go`
- Test: `internal/git/git_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/git/git_test.go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git command %v failed: %s: %v", args, out, err)
		}
	}
	// Create initial commit
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("test"), 0644)
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = dir
	cmd.Run()

	return dir
}

func TestIsGitRepo(t *testing.T) {
	repo := initTestRepo(t)
	if !IsGitRepo(repo) {
		t.Error("expected true for git repo")
	}

	nonRepo := t.TempDir()
	if IsGitRepo(nonRepo) {
		t.Error("expected false for non-git directory")
	}
}

func TestGetRemoteURL(t *testing.T) {
	repo := initTestRepo(t)
	// No remote set
	url := GetRemoteURL(repo)
	if url != "" {
		t.Errorf("expected empty remote URL, got %q", url)
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repo := initTestRepo(t)
	branch, err := GetCurrentBranch(repo)
	if err != nil {
		t.Fatalf("GetCurrentBranch() error: %v", err)
	}
	// Default branch is usually "main" or "master"
	if branch == "" {
		t.Error("expected non-empty branch name")
	}
}

func TestIsDirty_Clean(t *testing.T) {
	repo := initTestRepo(t)
	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty() error: %v", err)
	}
	if dirty {
		t.Error("expected clean repo")
	}
}

func TestIsDirty_WithChanges(t *testing.T) {
	repo := initTestRepo(t)
	os.WriteFile(filepath.Join(repo, "new.txt"), []byte("change"), 0644)

	dirty, err := IsDirty(repo)
	if err != nil {
		t.Fatalf("IsDirty() error: %v", err)
	}
	if !dirty {
		t.Error("expected dirty repo")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/git/ -v
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/git/git.go
package git

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func runGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func IsGitRepo(path string) bool {
	_, err := runGit(path, "rev-parse", "--git-dir")
	return err == nil
}

func GetRemoteURL(path string) string {
	url, err := runGit(path, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return url
}

func GetCurrentBranch(path string) (string, error) {
	branch, err := runGit(path, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return branch, nil
}

func IsDirty(path string) (bool, error) {
	out, err := runGit(path, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	return out != "", nil
}

func Fetch(path string) error {
	_, err := runGit(path, "fetch")
	if err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	return nil
}

func CommitsBehind(path string) (int, error) {
	branch, err := GetCurrentBranch(path)
	if err != nil {
		return 0, err
	}
	out, err := runGit(path, "rev-list", "--count", fmt.Sprintf("HEAD..origin/%s", branch))
	if err != nil {
		return 0, nil // no remote tracking branch
	}
	count, err := strconv.Atoi(out)
	if err != nil {
		return 0, nil
	}
	return count, nil
}

func PullFF(path string) error {
	_, err := runGit(path, "pull", "--ff-only")
	if err != nil {
		return fmt.Errorf("git pull --ff-only: %w", err)
	}
	return nil
}

func StashAndPull(path string) error {
	if _, err := runGit(path, "stash"); err != nil {
		return fmt.Errorf("git stash: %w", err)
	}
	if err := PullFF(path); err != nil {
		// Try to pop stash back on failure
		runGit(path, "stash", "pop")
		return err
	}
	runGit(path, "stash", "pop")
	return nil
}

func ForceReset(path string) error {
	branch, err := GetCurrentBranch(path)
	if err != nil {
		return err
	}
	_, err = runGit(path, "reset", "--hard", fmt.Sprintf("origin/%s", branch))
	if err != nil {
		return fmt.Errorf("git reset --hard: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/git/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: add git operations (fetch, pull, status, stash, reset)"
```

---

### Task 11: Linker — symlink management and health checks

**Files:**
- Create: `internal/linker/linker.go`
- Test: `internal/linker/linker_test.go`

- [ ] **Step 1: Write the test**

```go
// internal/linker/linker_test.go
package linker

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSymlink_NewTarget(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("test"), 0644)

	target := filepath.Join(t.TempDir(), "skills", "my-skill")

	err := CreateSymlink(src, target)
	if err != nil {
		t.Fatalf("CreateSymlink() error: %v", err)
	}

	linkDest, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}
	if linkDest != src {
		t.Errorf("symlink points to %q, want %q", linkDest, src)
	}
}

func TestCreateSymlink_CreatesParentDirs(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(t.TempDir(), "deep", "nested", "path", "skill")

	err := CreateSymlink(src, target)
	if err != nil {
		t.Fatalf("CreateSymlink() error: %v", err)
	}

	if _, err := os.Lstat(target); err != nil {
		t.Error("symlink was not created")
	}
}

func TestBackupAndReplace_ExistingDir(t *testing.T) {
	base := t.TempDir()

	// Create existing skill directory
	existingDir := filepath.Join(base, "my-skill")
	os.MkdirAll(existingDir, 0755)
	os.WriteFile(filepath.Join(existingDir, "SKILL.md"), []byte("old"), 0644)

	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("new"), 0644)

	err := BackupAndReplace(src, existingDir)
	if err != nil {
		t.Fatalf("BackupAndReplace() error: %v", err)
	}

	// Check backup exists
	backupPath := existingDir + ".skill.bak.zip"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("backup zip was not created")
	}

	// Check symlink was created
	linkDest, err := os.Readlink(existingDir)
	if err != nil {
		t.Fatalf("not a symlink: %v", err)
	}
	if linkDest != src {
		t.Errorf("symlink points to %q, want %q", linkDest, src)
	}
}

func TestCheckSymlink_Active(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("test"), 0644)

	target := filepath.Join(t.TempDir(), "skill")
	os.Symlink(src, target)

	status := CheckSymlink(target, src)
	if status != StatusActive {
		t.Errorf("status = %q, want %q", status, StatusActive)
	}
}

func TestCheckSymlink_Broken(t *testing.T) {
	target := filepath.Join(t.TempDir(), "skill")
	os.Symlink("/nonexistent/path", target)

	status := CheckSymlink(target, "/nonexistent/path")
	if status != StatusBroken {
		t.Errorf("status = %q, want %q", status, StatusBroken)
	}
}

func TestCheckSymlink_Orphaned(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nonexistent-symlink")
	status := CheckSymlink(target, "/some/source")
	if status != StatusOrphaned {
		t.Errorf("status = %q, want %q", status, StatusOrphaned)
	}
}

func TestRemoveSymlink(t *testing.T) {
	src := t.TempDir()
	target := filepath.Join(t.TempDir(), "skill")
	os.Symlink(src, target)

	err := RemoveSymlink(target)
	if err != nil {
		t.Fatalf("RemoveSymlink() error: %v", err)
	}

	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		t.Error("symlink still exists after removal")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/linker/ -v
```
Expected: FAIL

- [ ] **Step 3: Write implementation**

```go
// internal/linker/linker.go
package linker

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	StatusActive   = "active"
	StatusBroken   = "broken"
	StatusOrphaned = "orphaned"
)

func CreateSymlink(source, target string) error {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create parent dirs: %w", err)
	}
	return os.Symlink(source, target)
}

func RemoveSymlink(target string) error {
	return os.Remove(target)
}

func BackupAndReplace(source, target string) error {
	backupPath := target + ".skill.bak.zip"

	if err := zipDirectory(target, backupPath); err != nil {
		return fmt.Errorf("backup directory: %w", err)
	}

	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("remove original: %w", err)
	}

	return CreateSymlink(source, target)
}

func CheckSymlink(symlinkPath, expectedTarget string) string {
	fi, err := os.Lstat(symlinkPath)
	if os.IsNotExist(err) {
		return StatusOrphaned
	}
	if err != nil {
		return StatusBroken
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		return StatusBroken
	}

	dest, err := os.Readlink(symlinkPath)
	if err != nil {
		return StatusBroken
	}

	// Check if target still exists
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		return StatusBroken
	}

	return StatusActive
}

func IsSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink != 0
}

func IsDirectory(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fi.IsDir()
}

func ExistsAtTarget(target string) bool {
	_, err := os.Lstat(target)
	return err == nil
}

func zipDirectory(source, target string) error {
	zipFile, err := os.Create(target)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	w := zip.NewWriter(zipFile)
	defer w.Close()

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath := strings.TrimPrefix(path, filepath.Dir(source))
		relPath = strings.TrimPrefix(relPath, string(filepath.Separator))

		if info.IsDir() {
			_, err := w.Create(relPath + "/")
			return err
		}

		writer, err := w.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/linker/ -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/linker/
git commit -m "feat: add linker for symlink management, backup, and health checks"
```

---

## Chunk 4: TUI — Styles, Keys, App Shell, Main Menu

### Task 12: TUI styles and keybindings

**Files:**
- Create: `internal/tui/styles.go`
- Create: `internal/tui/keys.go`

- [ ] **Step 1: Write styles.go**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 2)

	warningStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF9900"))

	successStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00CC00"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	subtleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true)

	normalStyle = lipgloss.NewStyle()

	boxStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7D56F4")).
		Padding(1, 2)
)
```

- [ ] **Step 2: Write keys.go**

```go
// internal/tui/keys.go
package tui

import "github.com/charmbracelet/bubbles/key"

type globalKeyMap struct {
	Quit key.Binding
	Back key.Binding
	Help key.Binding
}

var globalKeys = globalKeyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Back: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "help"),
	),
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/styles.go internal/tui/keys.go
git commit -m "feat: add TUI styles and keybindings"
```

---

### Task 13: TUI app shell — root model with menu stack

**Files:**
- Create: `internal/tui/app.go`

- [ ] **Step 1: Write app.go**

This is the root BubbleTea model that manages a stack of views (main menu, skills, sources, providers). Each submenu is its own model.

```go
// internal/tui/app.go
package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
)

type view int

const (
	viewMain view = iota
	viewSkills
	viewSources
	viewProviders
)

type App struct {
	db        *db.Database
	current   view
	mainMenu  mainMenuModel
	skills    skillsMenuModel
	sources   sourcesMenuModel
	providers providersMenuModel
	width     int
	height    int
}

func NewApp(d *db.Database) App {
	return App{
		db:        d,
		current:   viewMain,
		mainMenu:  newMainMenu(d),
		skills:    newSkillsMenu(d),
		sources:   newSourcesMenu(d),
		providers: newProvidersMenu(d),
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(
		a.mainMenu.Init(),
		checkSourceUpdates(a.db),
	)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		// Global quit
		if key.Matches(msg, globalKeys.Quit) && a.current == viewMain {
			return a, tea.Quit
		}
		// Back to main from submenus
		if key.Matches(msg, globalKeys.Back) && a.current != viewMain {
			a.current = viewMain
			return a, nil
		}

	case navigateMsg:
		a.current = msg.target
		switch msg.target {
		case viewSkills:
			a.skills = newSkillsMenu(a.db)
		case viewSources:
			a.sources = newSourcesMenu(a.db)
		case viewProviders:
			a.providers = newProvidersMenu(a.db)
		}
		return a, nil
	}

	var cmd tea.Cmd
	switch a.current {
	case viewMain:
		a.mainMenu, cmd = a.mainMenu.update(msg)
	case viewSkills:
		a.skills, cmd = a.skills.update(msg)
	case viewSources:
		a.sources, cmd = a.sources.update(msg)
	case viewProviders:
		a.providers, cmd = a.providers.update(msg)
	}
	return a, cmd
}

func (a App) View() string {
	switch a.current {
	case viewSkills:
		return a.skills.view()
	case viewSources:
		return a.sources.view()
	case viewProviders:
		return a.providers.view()
	default:
		return a.mainMenu.view()
	}
}

// navigateMsg tells the app to switch to a different view
type navigateMsg struct {
	target view
}

func navigate(target view) tea.Cmd {
	return func() tea.Msg {
		return navigateMsg{target: target}
	}
}

// sourceUpdateMsg carries fetch results
type sourceUpdateMsg struct {
	sourceName   string
	commitsBehind int
	err          error
}

func checkSourceUpdates(d *db.Database) tea.Cmd {
	return func() tea.Msg {
		// This will be called once; it dispatches individual fetch commands
		// Implemented in menu_main.go via batch commands
		return nil
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat: add TUI app shell with menu stack navigation"
```

---

### Task 14: Main menu

**Files:**
- Create: `internal/tui/menu_main.go`

- [ ] **Step 1: Write menu_main.go**

```go
// internal/tui/menu_main.go
package tui

import (
	"fmt"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/git"
	"github.com/vladyslav/skillreg/internal/models"
)

type mainMenuItem struct {
	label  string
	target view
}

type mainMenuModel struct {
	db            *db.Database
	items         []mainMenuItem
	cursor        int
	updatesBanner string
	checking      bool
}

func newMainMenu(d *db.Database) mainMenuModel {
	return mainMenuModel{
		db: d,
		items: []mainMenuItem{
			{label: "Skills", target: viewSkills},
			{label: "Sources", target: viewSources},
			{label: "Providers", target: viewProviders},
		},
		checking: true,
	}
}

func (m mainMenuModel) Init() tea.Cmd {
	return m.fetchAllSources()
}

type fetchResultsMsg struct {
	results []sourceUpdateMsg
}

func (m mainMenuModel) fetchAllSources() tea.Cmd {
	return func() tea.Msg {
		sources, err := models.ListSources(m.db)
		if err != nil || len(sources) == 0 {
			return fetchResultsMsg{}
		}

		var wg sync.WaitGroup
		results := make([]sourceUpdateMsg, len(sources))
		for i, src := range sources {
			wg.Add(1)
			go func(idx int, s models.Source) {
				defer wg.Done()
				result := sourceUpdateMsg{sourceName: s.Name}
				if err := git.Fetch(s.Path); err != nil {
					result.err = err
				} else {
					behind, _ := git.CommitsBehind(s.Path)
					result.commitsBehind = behind
				}
				models.UpdateSourceLastChecked(m.db, s.ID)
				results[idx] = result
			}(i, src)
		}
		wg.Wait()
		return fetchResultsMsg{results: results}
	}
}

func (m mainMenuModel) update(msg tea.Msg) (mainMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case fetchResultsMsg:
		m.checking = false
		totalBehind := 0
		for _, r := range msg.results {
			if r.commitsBehind > 0 {
				totalBehind++
			}
		}
		if totalBehind > 0 {
			m.updatesBanner = fmt.Sprintf("%d source(s) have updates available", totalBehind)
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			return m, navigate(m.items[m.cursor].target)
		}
	}
	return m, nil
}

func (m mainMenuModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Skill Registry"))
	b.WriteString("\n\n")

	if m.checking {
		b.WriteString(subtleStyle.Render("  Checking sources for updates..."))
		b.WriteString("\n\n")
	} else if m.updatesBanner != "" {
		b.WriteString(warningStyle.Render("  ⚠ " + m.updatesBanner))
		b.WriteString("\n\n")
	}

	// Count items for display
	skillCount := countLabel(m.db, "skills")
	sourceCount := countLabel(m.db, "sources")
	providerCount := countLabel(m.db, "instances")

	counts := []string{skillCount, sourceCount, providerCount}

	for i, item := range m.items {
		cursor := "  "
		style := normalStyle
		if i == m.cursor {
			cursor = "→ "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s (%s)", cursor, item.label, counts[i])))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  ↑↓ navigate • enter select • q quit"))

	return b.String()
}

func countLabel(d *db.Database, table string) string {
	var count int
	d.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	return fmt.Sprintf("%d", count)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/menu_main.go
git commit -m "feat: add main menu with source update checking"
```

---

### Task 15: Entry point — cmd/skillreg/main.go

**Files:**
- Create: `cmd/skillreg/main.go`

- [ ] **Step 1: Write main.go**

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

func main() {
	d, err := db.Open(config.DBPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	if err := d.SeedProviders(); err != nil {
		fmt.Fprintf(os.Stderr, "Error seeding providers: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(d)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify build compiles**

Note: This will fail until stub submenu models exist. Create stubs first.

```bash
go build ./cmd/skillreg/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/skillreg/main.go
git commit -m "feat: add CLI entry point"
```

---

## Chunk 5: TUI — Submenu Stubs and Providers Menu

### Task 16: Stub submenu models (skills, sources, providers)

The app.go references `skillsMenuModel`, `sourcesMenuModel`, `providersMenuModel`. Create stubs so the project compiles, then flesh them out.

**Files:**
- Create: `internal/tui/menu_skills.go`
- Create: `internal/tui/menu_sources.go`
- Create: `internal/tui/menu_providers.go`

- [ ] **Step 1: Write stub menu_skills.go**

```go
// internal/tui/menu_skills.go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

type skillsMenuModel struct {
	db     *db.Database
	skills []models.Skill
	cursor int
}

func newSkillsMenu(d *db.Database) skillsMenuModel {
	skills, _ := models.ListAllSkills(d)
	return skillsMenuModel{db: d, skills: skills}
}

func (m skillsMenuModel) update(msg tea.Msg) (skillsMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.skills)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m skillsMenuModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Skills"))
	b.WriteString("\n\n")

	if len(m.skills) == 0 {
		b.WriteString(subtleStyle.Render("  No skills found. Add a source first."))
		b.WriteString("\n")
	} else {
		for i, skill := range m.skills {
			cursor := "  "
			style := normalStyle
			if i == m.cursor {
				cursor = "→ "
				style = selectedStyle
			}
			b.WriteString(style.Render(cursor + skill.Name))
			if skill.Description != "" {
				b.WriteString(subtleStyle.Render(" — " + skill.Description))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  esc back • q quit"))
	return b.String()
}
```

- [ ] **Step 2: Write stub menu_sources.go**

```go
// internal/tui/menu_sources.go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

type sourcesMenuModel struct {
	db      *db.Database
	sources []models.Source
	cursor  int
}

func newSourcesMenu(d *db.Database) sourcesMenuModel {
	sources, _ := models.ListSources(d)
	return sourcesMenuModel{db: d, sources: sources}
}

func (m sourcesMenuModel) update(msg tea.Msg) (sourcesMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sources) {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m sourcesMenuModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Sources"))
	b.WriteString("\n\n")

	if len(m.sources) == 0 {
		b.WriteString(subtleStyle.Render("  No sources registered."))
		b.WriteString("\n")
	} else {
		for i, src := range m.sources {
			cursor := "  "
			style := normalStyle
			if i == m.cursor {
				cursor = "→ "
				style = selectedStyle
			}
			b.WriteString(style.Render(cursor + src.Name))
			b.WriteString(subtleStyle.Render("  " + src.Path))
			b.WriteString("\n")
		}
	}

	// Add source option
	addCursor := "  "
	addStyle := normalStyle
	if m.cursor == len(m.sources) {
		addCursor = "→ "
		addStyle = selectedStyle
	}
	b.WriteString(addStyle.Render(addCursor + "[Add source]"))
	b.WriteString("\n\n")
	b.WriteString(subtleStyle.Render("  esc back • q quit"))
	return b.String()
}
```

- [ ] **Step 3: Write stub menu_providers.go**

```go
// internal/tui/menu_providers.go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

type providerWithInstances struct {
	provider  models.Provider
	instances []models.Instance
}

type providersMenuModel struct {
	db        *db.Database
	providers []providerWithInstances
	cursor    int
}

func newProvidersMenu(d *db.Database) providersMenuModel {
	providers, _ := models.ListProviders(d)
	var pwi []providerWithInstances
	for _, p := range providers {
		instances, _ := models.ListInstancesByProvider(d, p.ID)
		pwi = append(pwi, providerWithInstances{provider: p, instances: instances})
	}
	return providersMenuModel{db: d, providers: pwi}
}

func (m providersMenuModel) update(msg tea.Msg) (providersMenuModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			max := m.totalRows() - 1
			if m.cursor < max {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m providersMenuModel) totalRows() int {
	count := 0
	for _, p := range m.providers {
		count++ // provider row
		count += len(p.instances)
		count++ // [Add instance] row
	}
	return count
}

func (m providersMenuModel) view() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Providers"))
	b.WriteString("\n\n")

	// Check if we should offer scan
	allInstances, _ := models.ListAllInstances(m.db)
	if len(allInstances) == 0 {
		b.WriteString(warningStyle.Render("  No instances configured."))
		b.WriteString("\n")
		b.WriteString(subtleStyle.Render("  Use [Add instance] under a provider to get started."))
		b.WriteString("\n\n")
	}

	row := 0
	for _, pwi := range m.providers {
		cursor := "  "
		style := normalStyle
		if row == m.cursor {
			cursor = "→ "
			style = selectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s (%s)", cursor, pwi.provider.Name, pwi.provider.ConfigDirPrefix)))
		b.WriteString("\n")
		row++

		for _, inst := range pwi.instances {
			cursor = "    "
			style = normalStyle
			if row == m.cursor {
				cursor = "  → "
				style = selectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, inst.Name)))
			b.WriteString(subtleStyle.Render("  " + inst.GlobalSkillsPath))
			b.WriteString("\n")
			row++
		}

		cursor = "    "
		style = subtleStyle
		if row == m.cursor {
			cursor = "  → "
			style = selectedStyle
		}
		b.WriteString(style.Render(cursor + "[Add instance]"))
		b.WriteString("\n")
		row++
	}

	b.WriteString("\n")
	b.WriteString(subtleStyle.Render("  esc back • q quit"))
	return b.String()
}
```

- [ ] **Step 4: Verify project compiles**

```bash
go build ./cmd/skillreg/
```
Expected: builds successfully

- [ ] **Step 5: Commit**

```bash
git add internal/tui/menu_skills.go internal/tui/menu_sources.go internal/tui/menu_providers.go
git commit -m "feat: add TUI submenu stubs (skills, sources, providers)"
```

---

## Chunk 6: TUI — Full Interactive Menus

### Task 17: Sources menu — add source with text input

**Files:**
- Modify: `internal/tui/menu_sources.go`

- [ ] **Step 1: Enhance menu_sources.go with add-source flow**

Replace the stub with full implementation that includes:
- List sources with status (up to date / N new commits)
- Select source → submenu: pull, rescan, toggle auto-update, remove
- Add source: text input for path, validate git repo, scan for skills, confirm
- Use `textinput` bubble for path entry
- Use git.Fetch/CommitsBehind for status display

The implementation should handle these states as an enum:
- `sourcesViewList` — main source list
- `sourcesViewDetail` — selected source actions
- `sourcesViewAddPath` — text input for new source path
- `sourcesViewAddConfirm` — show discovered skills, confirm add
- `sourcesViewPullOptions` — dirty/conflict resolution options

Key interactions:
- Enter on a source → detail view with actions
- Enter on [Add source] → text input for path
- In detail: pull, rescan, toggle auto-update, remove (with confirmation)

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/skillreg/
```

- [ ] **Step 3: Manual test — run and navigate to Sources**

```bash
go run ./cmd/skillreg/
```
Navigate to Sources, try adding a source path.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/menu_sources.go
git commit -m "feat: implement full sources menu with add/pull/manage flows"
```

---

### Task 18: Skills menu — browse, install, uninstall

**Files:**
- Modify: `internal/tui/menu_skills.go`

- [ ] **Step 1: Enhance menu_skills.go with full implementation**

Replace the stub with full implementation that includes:
- Browse all: table view with skill name, source name, install status
- Same-name disambiguation: show source in parentheses
- Install flow: select skill → multi-select instances → collision detection → symlink
- Uninstall flow: show installed skills → multi-select → remove symlinks
- Use `table` bubble for browse view
- Use `list` bubble with filtering for skill selection

States:
- `skillsViewBrowse` — table of all skills
- `skillsViewInstallSelect` — pick a skill to install
- `skillsViewInstallTargets` — multi-select target instances
- `skillsViewUninstallSelect` — pick installed skills to remove
- `skillsViewCollision` — collision resolution (pick/rename)

Key interactions:
- Browse shows all skills with where they're installed
- Install: fuzzy filter skills → select instances (with "all" shortcuts) → handle collisions → create symlinks
- Uninstall: select from installed → confirm → remove symlinks

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/skillreg/
```

- [ ] **Step 3: Manual test — run and navigate to Skills**

```bash
go run ./cmd/skillreg/
```
Add a source first, then try browsing and installing skills.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/menu_skills.go
git commit -m "feat: implement full skills menu with browse/install/uninstall"
```

---

### Task 19: Providers menu — add instance with home scan

**Files:**
- Modify: `internal/tui/menu_providers.go`

- [ ] **Step 1: Enhance menu_providers.go with full implementation**

Replace the stub with full implementation that includes:
- List providers with nested instances
- Enter on provider → not interactive (just visual grouping)
- Enter on instance → show installed skills, option to remove
- Enter on [Add instance] → text input for name and path, or home directory scan
- Home scan: glob `~/.<prefix>*/` for matching dirs, suggest instance names
- Collision check on `global_skills_path` uniqueness

States:
- `providersViewList` — tree view
- `providersViewInstanceDetail` — selected instance info
- `providersViewAddInstance` — text inputs for name/path
- `providersViewScanHome` — auto-discovered instances with checkboxes

Key interactions:
- When zero instances exist and entering providers: offer home scan
- Add instance: pick provider → enter name → enter/confirm path
- Home scan: find `~/.<prefix>*/` directories → multi-select → create instances

- [ ] **Step 2: Verify build**

```bash
go build ./cmd/skillreg/
```

- [ ] **Step 3: Manual test**

```bash
go run ./cmd/skillreg/
```
Navigate to Providers, try home scan and adding instances.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/menu_providers.go
git commit -m "feat: implement full providers menu with home scan and instance management"
```

---

### Task 20: Confirm dialog component

**Files:**
- Create: `internal/tui/components/confirm.go`

- [ ] **Step 1: Write confirm.go**

```go
// internal/tui/components/confirm.go
package components

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ConfirmModel struct {
	Question string
	Focused  int // 0=yes, 1=no
	Decided  bool
	Result   bool
}

func NewConfirm(question string) ConfirmModel {
	return ConfirmModel{Question: question, Focused: 1} // default to No
}

type ConfirmResultMsg struct {
	Confirmed bool
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.Focused = 0
		case "right", "l":
			m.Focused = 1
		case "y":
			m.Decided = true
			m.Result = true
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: true} }
		case "n", "esc":
			m.Decided = true
			m.Result = false
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: false} }
		case "enter":
			m.Decided = true
			m.Result = m.Focused == 0
			return m, func() tea.Msg { return ConfirmResultMsg{Confirmed: m.Result} }
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	yesStyle := lipgloss.NewStyle().Padding(0, 2)
	noStyle := lipgloss.NewStyle().Padding(0, 2)

	if m.Focused == 0 {
		yesStyle = yesStyle.Bold(true).Foreground(lipgloss.Color("#00CC00"))
	} else {
		noStyle = noStyle.Bold(true).Foreground(lipgloss.Color("#FF0000"))
	}

	return fmt.Sprintf("%s\n\n%s%s",
		m.Question,
		yesStyle.Render("[Yes]"),
		noStyle.Render("[No]"),
	)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/components/confirm.go
git commit -m "feat: add confirmation dialog component"
```

---

### Task 21: Status bar component

**Files:**
- Create: `internal/tui/components/statusbar.go`

- [ ] **Step 1: Write statusbar.go**

```go
// internal/tui/components/statusbar.go
package components

import (
	"github.com/charmbracelet/lipgloss"
)

var statusBarStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FFFDF5")).
	Background(lipgloss.Color("#353533")).
	Padding(0, 1)

func StatusBar(text string, width int) string {
	return statusBarStyle.Width(width).Render(text)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/components/statusbar.go
git commit -m "feat: add status bar component"
```

---

## Chunk 7: Integration, Build, Release, and Documentation

### Task 22: End-to-end integration test

**Files:**
- Create: `internal/integration_test.go`

- [ ] **Step 1: Write integration test**

```go
// internal/integration_test.go
package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/scanner"
)

func TestFullWorkflow(t *testing.T) {
	// 1. Set up DB
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	d.SeedProviders()

	// 2. Create a fake source repo with skills
	sourceDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = sourceDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceDir
	cmd.Run()

	// Create two skills
	skillDir1 := filepath.Join(sourceDir, "skill-a")
	os.MkdirAll(skillDir1, 0755)
	os.WriteFile(filepath.Join(skillDir1, "SKILL.md"), []byte("---\ndescription: Skill A\n---\n# Skill A"), 0644)

	skillDir2 := filepath.Join(sourceDir, "skill-b")
	os.MkdirAll(skillDir2, 0755)
	os.WriteFile(filepath.Join(skillDir2, "SKILL.md"), []byte("# Skill B\nA useful skill"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = sourceDir
	cmd.Run()

	// 3. Register source
	src, err := models.CreateSource(d, "test-source", sourceDir, "")
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	// 4. Scan for skills
	discovered, err := scanner.ScanRepo(sourceDir)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(discovered) != 2 {
		t.Fatalf("discovered %d skills, want 2", len(discovered))
	}

	// 5. Store skills
	for _, ds := range discovered {
		models.CreateSkill(d, src.ID, ds.Name, ds.Path, ds.Description)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 2 {
		t.Fatalf("stored %d skills, want 2", len(skills))
	}

	// 6. Create an instance
	providers, _ := models.ListProviders(d)
	instanceDir := filepath.Join(t.TempDir(), "skills")
	inst, err := models.CreateInstance(d, providers[0].ID, "test-instance", instanceDir, true)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}

	// 7. Install a skill (create symlink)
	targetPath := filepath.Join(inst.GlobalSkillsPath, skills[0].Name)
	err = linker.CreateSymlink(skills[0].OriginalPath, targetPath)
	if err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	installation, err := models.CreateInstallation(d, skills[0].ID, inst.ID, targetPath, skills[0].Name)
	if err != nil {
		t.Fatalf("create installation: %v", err)
	}

	// 8. Verify symlink
	status := linker.CheckSymlink(targetPath, skills[0].OriginalPath)
	if status != linker.StatusActive {
		t.Errorf("symlink status = %q, want %q", status, linker.StatusActive)
	}

	// 9. Uninstall
	err = linker.RemoveSymlink(targetPath)
	if err != nil {
		t.Fatalf("remove symlink: %v", err)
	}
	models.DeleteInstallation(d, installation.ID)

	// Verify orphaned
	status = linker.CheckSymlink(targetPath, skills[0].OriginalPath)
	if status != linker.StatusOrphaned {
		t.Errorf("after removal, status = %q, want %q", status, linker.StatusOrphaned)
	}
}
```

- [ ] **Step 2: Run integration test**

```bash
go test ./internal/ -v -run TestFullWorkflow
```
Expected: PASS

- [ ] **Step 3: Run all tests**

```bash
go test ./... -v
```
Expected: all PASS

- [ ] **Step 4: Commit**

```bash
git add internal/integration_test.go
git commit -m "test: add end-to-end integration test"
```

---

### Task 23: GoReleaser config

**Files:**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: Write .goreleaser.yaml**

```yaml
# .goreleaser.yaml
version: 2

builds:
  - main: ./cmd/skillreg
    binary: skillreg
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        format: zip

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
```

- [ ] **Step 2: Commit**

```bash
git add .goreleaser.yaml
git commit -m "feat: add GoReleaser configuration"
```

---

### Task 24: GitHub Actions release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write release.yml**

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.22"

      - name: Run tests
        run: go test ./... -v

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat: add GitHub Actions release workflow"
```

---

### Task 25: Create GitHub repository

- [ ] **Step 1: Create the repository using gh CLI**

```bash
cd /Users/vladyslav/Projects/SkillRegistry
gh repo create skillreg --public --description "Agent Skill Management CLI — manage AI coding assistant skills across providers via symlinks" --source=. --push
```

This will:
- Create the `skillreg` repo on GitHub under your account
- Set the origin remote
- Push all commits

- [ ] **Step 2: Verify the repo was created**

```bash
gh repo view --web
```

---

### Task 26: README.md

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README.md**

```markdown
# SkillReg

Agent Skill Management CLI — manage AI coding assistant skills across providers via symlinks.

SkillReg solves the problem of manually copying and maintaining skills across different AI coding assistant config directories (Claude, Codex, Gemini, Cursor, VSCode/Copilot, Antigravity).

## How it works

1. **Sources** — Register local git repos that contain skills (any directory with a `SKILL.md`)
2. **Providers & Instances** — Configure your agent installations (e.g., `claude-personal`, `claude-work`)
3. **Install** — Symlink skills from sources to any combination of instances
4. **Sync** — Check for upstream updates and re-scan sources

Skills are installed as symlinks, so updates to the source repo are immediately reflected everywhere.

## Install

Download the latest binary from [GitHub Releases](../../releases).

Or build from source:

```bash
go build -o skillreg ./cmd/skillreg
```

## Usage

```bash
skillreg
```

This launches the interactive TUI. Navigate with arrow keys, select with Enter, go back with Esc, quit with Q.

### Main Menu

- **Skills** — Browse all discovered skills, install/uninstall to instances
- **Sources** — Add/remove git repos as skill sources, pull updates
- **Providers** — Manage agent providers and their instances

### Supported Providers

| Provider | Config directory |
|---|---|
| Claude | `~/.claude*/skills/` |
| Codex | `~/.agents/skills/` |
| Gemini | `~/.gemini*/skills/` |
| Cursor | `~/.cursor*/skills/` |
| VSCode / Copilot | `~/.github*/skills/` |
| Antigravity | `~/.agents/skills/` |

### Adding a Source

Navigate to Sources → [Add source] and enter the path to a local git repo. SkillReg will scan it recursively for directories containing `SKILL.md` files.

### Installing a Skill

Navigate to Skills → Install → select a skill → select target instances. SkillReg creates symlinks from the instance's skills directory to the source repo.

### Aliased Instances

Support multiple instances per provider. For example, if you have separate Claude configs for work and personal use:

- `claude-personal` → `~/.claude-personal/skills/`
- `claude-work` → `~/.claude-work/skills/`

## Data Storage

Runtime data (SQLite database) is stored at `~/.local/share/skillreg/skillreg.db` (respects `$XDG_DATA_HOME`).

## Development

```bash
# Run
go run ./cmd/skillreg

# Test
go test ./...

# Build
go build -o skillreg ./cmd/skillreg
```

## Release

Tag a version and push to trigger the GitHub Actions release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## License

MIT
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage guide and provider table"
```

---

### Task 27: AGENTS.md

**Files:**
- Create: `AGENTS.md`

- [ ] **Step 1: Write AGENTS.md**

```markdown
# SkillReg — Agent Instructions

## Project Overview

SkillReg is a Go CLI tool with interactive TUI (BubbleTea) for managing AI coding assistant skills across multiple providers. It uses SQLite for state and symlinks for skill installation.

## Architecture

- **cmd/skillreg/main.go** — Entry point. Opens DB, seeds providers, launches TUI.
- **internal/config/** — XDG paths, constants (excluded dirs, skills dir name).
- **internal/db/** — SQLite connection, migrations, provider seeding.
- **internal/models/** — CRUD for sources, providers, instances, skills, installations.
- **internal/scanner/** — Recursively scans repos for `SKILL.md` files, parses descriptions.
- **internal/git/** — Git operations: fetch, pull, stash, reset, status checks.
- **internal/linker/** — Symlink creation/removal, backup (zip), health checks.
- **internal/tui/** — BubbleTea TUI layer with menu stack navigation.

## Key Conventions

- **TUI is decoupled from core logic.** Models, scanner, git, and linker are independently testable.
- **BubbleTea v1 stable** — Uses `github.com/charmbracelet/bubbletea` (not v2 beta).
- **Pure Go SQLite** — `modernc.org/sqlite`, no CGo required.
- **Skills dir is always `skills/`** — Hardcoded constant, not per-provider.
- **Paths are absolute** — `global_skills_path` stores tilde-expanded absolute paths.

## Testing

```bash
go test ./...
```

- Unit tests for each package in `*_test.go` files
- Integration test in `internal/integration_test.go`
- Tests use `t.TempDir()` for isolated filesystems

## Common Tasks

### Adding a new provider

1. Add entry to `DefaultProviders` in `internal/db/migrations.go`
2. The provider will be seeded on next run via `SeedProviders()`

### Adding a new model field

1. Add migration SQL to `migrations` slice in `internal/db/migrations.go`
2. Update the model struct and CRUD functions in `internal/models/`
3. Update tests

### Modifying TUI menus

- Each menu is in its own file: `menu_main.go`, `menu_skills.go`, `menu_sources.go`, `menu_providers.go`
- Navigation between menus uses `navigateMsg` and the menu stack in `app.go`
- Reusable components are in `internal/tui/components/`
```

- [ ] **Step 2: Commit**

```bash
git add AGENTS.md
git commit -m "docs: add AGENTS.md with project architecture and conventions"
```

---

### Task 28: Push to remote and verify

- [ ] **Step 1: Push all commits**

```bash
git push origin main
```

- [ ] **Step 2: Verify on GitHub**

```bash
gh repo view --web
```

- [ ] **Step 3: Run full test suite one final time**

```bash
go test ./... -v
```
Expected: all PASS
