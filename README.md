# SkillRegistry

## The Problem

AI-assisted development is fragmenting fast. Your team uses Claude Code, a colleague swears by Gemini CLI, half the frontend squad is on Cursor, and someone just introduced Codex. Each tool has its own config directory, its own way of loading custom skills, and its own conventions.

Now multiply that by the skills themselves. You have a repo of shared team prompts, a personal collection of coding patterns, open-source skill packs you pulled from GitHub — scattered across multiple repositories. Getting a single skill into a single tool means knowing where that tool looks for configs and manually copying or symlinking files there. Getting that same skill into three tools across two machines? Good luck.

**What you end up with:**

- Skills duplicated across provider directories, drifting out of sync
- New team members spending hours figuring out which skills go where
- Repository restructurings silently breaking installed skills
- No single view of what's installed, where, or from which source
- Switching or adding a new AI tool means repeating the entire setup

## The Solution

**SkillRegistry** is a terminal UI that treats skills as first-class, provider-agnostic packages. Point it at your skill repositories, tell it which AI tools you use, and it handles the rest — discovery, installation, syncing, and cleanup — through managed symlinks.

```
Sources (git repos)  →  SkillRegistry  →  Providers (Claude, Codex, Gemini, Cursor, ...)
                          ↕
                     SQLite database
                   (single source of truth)
```

**Three concepts, one workflow:**

1. **Sources** — Register local Git repositories containing skills. SkillRegistry scans them, parses metadata, and keeps track of changes when repos are restructured.
2. **Providers & Instances** — Define your AI tools and their config directories. Support multiple instances of the same provider (e.g., "Claude-Personal" and "Claude-Work").
3. **Install** — Browse discovered skills, pick a provider instance, and SkillRegistry creates a symlink. Uninstall removes it. That's it.

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

## Usage

Launch the interactive TUI:

```bash
skillreg
```

### Main Menu

The main menu provides navigation to:

- **Manage Sources** — Add/remove skill repositories and scan for new skills
- **Manage Providers** — Register AI tools and their config directories
- **Manage Provider Instances** — Create and configure specific tool instances
- **Browse Skills** — View discovered skills and install/remove them
- **Manage Installations** — View and manage active skill installations

## Supported Providers

| Provider | Config Directory |
|----------|------------------|
| Claude | `~/.claude` |
| Codex | `~/.agents` |
| Gemini | `~/.gemini` |
| Cursor | `~/.cursor` |
| VSCode Copilot | `~/.github` |
| Antigravity | `~/.gemini*/antigravity` |

## Adding Skills

### 1. Add a Source Repository

1. Open **Manage Sources**
2. Select **Add Source**
3. Enter the local path to a Git repository containing skills
4. Optionally scan to discover skills immediately

### 2. Install a Skill

1. Open **Browse Skills**
2. Select a skill to view details
3. Choose **Install to Instance**
4. Select a provider instance to install the skill
5. SkillRegistry creates a symlink to integrate the skill

### 3. Configure Aliased Instances

For tools supporting multiple configurations (e.g., Claude with different API keys or models):

1. Open **Manage Provider Instances**
2. Create a new instance with a custom alias (e.g., "Claude-Fast")
3. Configure the instance-specific settings
4. Install skills to each instance independently

## Data Storage

SkillRegistry stores all data in an XDG-compliant location:

- **Database:** `~/.local/share/skillreg/skillreg.db` (or `$XDG_DATA_HOME/skillreg/skillreg.db`)
- **Symlinks:** Placed in provider-specific directories as configured

## Development

### Run the Application

```bash
go run ./cmd/skillreg
```

### Run Tests

```bash
go test ./... -v
```

### Build Binary

```bash
go build -o skillreg ./cmd/skillreg
```

## Release

To create and publish a new release:

```bash
# Tag the commit
git tag -a v1.0.0 -m "Release v1.0.0"

# Push tag to trigger GitHub Actions
git push origin v1.0.0
```

GoReleaser will automatically:
- Run tests
- Build binaries for Linux, macOS, and Windows
- Create checksums
- Draft release notes on GitHub

## License

MIT License. See LICENSE file for details.
