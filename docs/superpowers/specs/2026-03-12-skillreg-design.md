# SkillReg — Agent Skill Management CLI

## Overview

SkillReg is a Go CLI tool with an interactive TUI for managing agent skills across multiple AI coding assistant providers. It solves the problem of manually copying and maintaining skills across different provider config directories.

**Core concept:** Git repositories are registered as "sources." Skills (directories containing `SKILL.md`) are discovered by scanning sources recursively. Skills are installed to provider instances via symlinks pointing back to the source repo. A SQLite database tracks all state.

## Technology Stack

- **Language:** Go
- **TUI framework:** BubbleTea + Lipgloss + Bubbles (Charm stack)
- **Database:** SQLite via `modernc.org/sqlite` (pure Go, no CGo)
- **Git operations:** Shell out to `git` CLI
- **Release:** GoReleaser + GitHub Actions
- **Binary name:** `skillreg`

## Data Model

### Tables

**sources** — Git repositories containing skills

| Column | Type | Description |
|---|---|---|
| id | INTEGER PK | Auto-increment |
| name | TEXT | Display name (e.g., "superpowers") |
| path | TEXT | Local filesystem path to git repo |
| remote_url | TEXT | Git remote URL (for display/reference) |
| auto_update | BOOLEAN | Whether to auto-pull on startup |
| last_checked_at | TIMESTAMP | Last fetch timestamp |
| created_at | TIMESTAMP | When source was added |

**providers** — Agent platform definitions

| Column | Type | Description |
|---|---|---|
| id | INTEGER PK | Auto-increment |
| name | TEXT | e.g., "Claude", "Codex", "Gemini" |
| config_dir_prefix | TEXT | e.g., ".claude", ".agents", ".gemini" |
| is_builtin | BOOLEAN | Default providers vs user-added |

**instances** — Specific provider installations (including aliases)

| Column | Type | Description |
|---|---|---|
| id | INTEGER PK | Auto-increment |
| provider_id | INTEGER FK | References providers |
| name | TEXT | e.g., "claude-personal", "claude-work" |
| global_skills_path | TEXT | Resolved absolute path, e.g., `~/.claude-personal/skills` |
| is_default | BOOLEAN | The "main" instance for this provider |
| created_at | TIMESTAMP | When instance was added |

**skills** — Discovered skills from sources

| Column | Type | Description |
|---|---|---|
| id | INTEGER PK | Auto-increment |
| source_id | INTEGER FK | References sources |
| name | TEXT | e.g., "brainstorming" |
| original_path | TEXT | Full path in source repo |
| description | TEXT | Parsed from SKILL.md |
| discovered_at | TIMESTAMP | When skill was found |

**installations** — Symlink tracking

| Column | Type | Description |
|---|---|---|
| id | INTEGER PK | Auto-increment |
| skill_id | INTEGER FK | References skills |
| instance_id | INTEGER FK | References instances |
| symlink_path | TEXT | Actual symlink path created |
| installed_name | TEXT | Name used (may differ from skill.name if renamed) |
| installed_at | TIMESTAMP | When installed |
| status | TEXT | "active", "broken", "orphaned" |

### Hardcoded Constants

- Skills subdirectory: `skills/` — universal across all providers, not stored in DB

## Default Providers

Seeded on first run:

| Provider | Config dir prefix |
|---|---|
| Claude | `.claude` |
| Codex | `.agents` |
| Gemini | `.gemini` |
| Cursor | `.cursor` |
| VSCode / Copilot | `.github` |
| Antigravity | `.agents` |

No instances are created automatically. On first visit to Providers menu, offer to scan `~/` for existing config directories and register them as instances.

## Application Structure

```
skillreg/
├── cmd/
│   └── skillreg/
│       └── main.go              -- entry point
├── internal/
│   ├── db/
│   │   ├── db.go                -- SQLite connection, migrations
│   │   └── migrations/          -- SQL migration files
│   ├── models/
│   │   ├── source.go
│   │   ├── provider.go
│   │   ├── instance.go
│   │   ├── skill.go
│   │   └── installation.go
│   ├── scanner/
│   │   └── scanner.go           -- scan source repos for SKILL.md files
│   ├── git/
│   │   └── git.go               -- fetch, pull, status checks
│   ├── linker/
│   │   └── linker.go            -- symlink creation/removal/health checks
│   └── tui/
│       ├── app.go               -- root BubbleTea model
│       ├── menu_main.go         -- main menu (Skills, Sources, Providers)
│       ├── menu_skills.go       -- skills submenu
│       ├── menu_sources.go      -- sources submenu
│       ├── menu_providers.go    -- providers submenu with instances
│       ├── components/          -- reusable TUI components
│       └── styles.go            -- Lipgloss styles/theme
├── data/
│   └── skillreg.db              -- SQLite database (gitignored)
├── .github/
│   └── workflows/
│       └── release.yml          -- GoReleaser build + GitHub Release
├── go.mod
├── go.sum
└── .gitignore
```

## TUI Flow

### Startup Sequence

1. Initialize DB (run migrations if needed)
2. Seed default providers if first run
3. Background `git fetch` on all sources
4. For sources with `auto_update: true` — auto `git pull` + rescan
5. Show main menu with update notifications

### Main Menu

```
┌─────────────────────────────────────┐
│  Skill Registry                     │
│                                     │
│  ⚠ 2 sources have updates          │
│                                     │
│  → Skills (34 available)            │
│    Sources (4 registered)           │
│    Providers (3 configured)         │
│                                     │
│  q quit                             │
└─────────────────────────────────────┘
```

### Skills Menu

**Browse all** — table with skill name, source, and where installed. Filterable. Enter for details.

**Install skill:**
1. Fuzzy-searchable skill list
2. Multi-select target instances (with "all Claude", "all instances" shortcuts)
3. Collision detection:
   - If target exists as a plain directory: warn, offer to backup as `<name>.skill.bak.zip` and replace with symlink
   - If target exists as a symlink to a different source: warn, let user pick which source or rename
   - Same-name skill from different source: warn, let user pick or rename
4. Create symlinks, record in DB

**Uninstall skill:**
1. Show only installed skills
2. Multi-select which installations to remove
3. Remove symlinks, update DB

### Sources Menu

List all sources with status (up to date / N new commits / unavailable). Select a source to:
- Pull updates (with options for dirty/conflict states)
- Rescan skills
- Toggle auto-update
- Remove source

**Add source:** Enter path to local git repo directory. Validate it's a git repo, scan for skills, display found skills, confirm add.

### Providers Menu

List providers with their instances nested underneath. Select a provider to manage instances. Select an instance to see installed skills or remove it.

**Add instance:** Select provider, enter instance name, enter or confirm config directory path.

## Skill Discovery

Recursive scan of source repos for any directory containing a `SKILL.md` file. Scans all directories including dot-prefixed ones (`.claude/`, `.github/`, `.agents/`, etc.).

All discovered skills are treated as universal — installable to any provider instance regardless of their original location in the source repo.

The skill name is derived from the immediate parent directory of `SKILL.md`.

## Symlink Management

### Creating Symlinks

Target path: `<instance.global_skills_path>/<skill_name>`

Example:
```
~/.claude-personal/skills/brainstorming → /Users/vladyslav/Projects/superpowers/skills/brainstorming
```

- Create parent directories if they don't exist
- On collision with plain directory: backup as `<name>.skill.bak.zip`, replace with symlink
- On collision with existing symlink: warn + pick/rename

### Health Checks

On startup or manual trigger:
- Iterate all installations in DB
- Check each symlink: exists? points to correct target? target still exists?
- Mark status: `active`, `broken` (dangling), `orphaned` (removed outside skillreg)
- Surface broken/orphaned in the UI with repair options

## Git Operations

### Fetch (startup, non-destructive)

For each source, run `git fetch`. Compare `HEAD` vs `origin/<branch>`, count commits behind. Display notification in main menu.

### Pull (manual or auto-update)

1. Attempt `git pull --ff-only`
2. On success: rescan for new/removed skills, update DB
3. On dirty working tree, offer options:
   - Stash changes and pull
   - Skip this source
   - Open terminal at source directory
4. On merge conflict / non-fast-forward, offer options:
   - Force pull (reset to remote)
   - Skip this source
   - Open terminal at source directory

### Post-Pull

- Detect new skills → add to DB
- Detect removed skills → mark installations as broken, notify user
- Existing symlinks remain valid (they point into the updated source directory)

## GitHub Actions & Release

**Trigger:** Tag push matching `v*`

**Build matrix:** `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`

**Tool:** GoReleaser — handles cross-compilation, checksums, GitHub Release creation

**Install methods:**
- Download binary from GitHub Releases
- `go install github.com/<owner>/skillreg@latest`

## Scope

### In scope (v1)
- Global skill management only (home directory config dirs)
- Interactive TUI menu
- Source repo management with git fetch/pull
- Symlink-based skill installation
- SQLite state tracking
- Collision handling with rename support
- Health checks for broken/orphaned symlinks
- Cross-platform binary releases

### Out of scope (future)
- Project-level skill management
- Homebrew tap
- Remote source repos (clone from URL)
- Skill dependency management
- Skill authoring/scaffolding
