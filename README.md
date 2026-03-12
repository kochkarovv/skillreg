# SkillRegistry

SkillRegistry is a terminal user interface (TUI) for discovering, organizing, and managing custom skills across AI tools. It sources skills from local Git repositories, organizes them by AI provider, and creates intelligent symlinks to integrate them seamlessly with your development environment.

## How It Works

SkillRegistry follows a three-step workflow:

1. **Sources** — Add local Git repositories containing skills (custom prompts, functions, or configurations)
2. **Providers & Instances** — Define which AI tools (Claude, Codex, Gemini, etc.) and their specific configurations should use skills
3. **Symlinks** — Create managed symlinks from provider config directories to installed skills, enabling automatic discovery

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
