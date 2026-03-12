package db

var migrations = []string{
	// sources table - stores configured skill source repositories
	`CREATE TABLE IF NOT EXISTS sources (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		path TEXT NOT NULL UNIQUE,
		remote_url TEXT NOT NULL DEFAULT '',
		auto_update BOOLEAN NOT NULL DEFAULT 0,
		last_checked_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,

	// providers table - stores supported skill providers (Claude, Codex, Gemini, etc.)
	`CREATE TABLE IF NOT EXISTS providers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		config_dir_prefix TEXT NOT NULL,
		is_builtin BOOLEAN NOT NULL DEFAULT 0
	)`,

	// instances table - stores individual provider installations
	`CREATE TABLE IF NOT EXISTS instances (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		provider_id INTEGER NOT NULL REFERENCES providers(id),
		name TEXT NOT NULL UNIQUE,
		global_skills_path TEXT NOT NULL UNIQUE,
		is_default BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,

	// skills table - discovered skills from sources
	`CREATE TABLE IF NOT EXISTS skills (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id INTEGER NOT NULL REFERENCES sources(id),
		name TEXT NOT NULL,
		original_path TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		discovered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source_id, original_path)
	)`,

	// installations table - skills installed in specific instances
	`CREATE TABLE IF NOT EXISTS installations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		skill_id INTEGER NOT NULL REFERENCES skills(id),
		instance_id INTEGER NOT NULL REFERENCES instances(id),
		symlink_path TEXT NOT NULL,
		installed_name TEXT NOT NULL,
		installed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		status TEXT NOT NULL DEFAULT 'active',
		UNIQUE(skill_id, instance_id)
	)`,
}

// DefaultProvider represents a built-in provider with its config directory prefix.
type DefaultProvider struct {
	Name              string
	ConfigDirPrefix   string
}

// DefaultProviders contains all default providers.
var DefaultProviders = []DefaultProvider{
	{Name: "Claude", ConfigDirPrefix: ".claude"},
	{Name: "Codex", ConfigDirPrefix: ".agents"},
	{Name: "Gemini", ConfigDirPrefix: ".gemini"},
	{Name: "Cursor", ConfigDirPrefix: ".cursor"},
	{Name: "VSCode Copilot", ConfigDirPrefix: ".github"},
	{Name: "Antigravity", ConfigDirPrefix: ".gemini*/antigravity"},
}
