package config

import (
	"os"
	"path/filepath"
)

const SkillsDirName = "skills"

var ExcludedDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"vendor":      true,
	"__pycache__": true,
	".venv":       true,
}

// DataDir returns the XDG data directory for skillreg.
// Uses $XDG_DATA_HOME if set, otherwise defaults to ~/.local/share/skillreg
func DataDir() string {
	xdgDataHome := os.Getenv("XDG_DATA_HOME")
	if xdgDataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to current working directory if home cannot be determined
			home = "."
		}
		xdgDataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(xdgDataHome, "skillreg")
}

// DBPath returns the full path to the skillreg SQLite database.
func DBPath() string {
	return filepath.Join(DataDir(), "skillreg.db")
}
