package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSkillsDirName(t *testing.T) {
	if SkillsDirName != "skills" {
		t.Errorf("SkillsDirName = %q, want %q", SkillsDirName, "skills")
	}
}

func TestExcludedDirs(t *testing.T) {
	expectedDirs := map[string]bool{
		".git":        true,
		"node_modules": true,
		"vendor":      true,
		"__pycache__": true,
		".venv":       true,
	}

	for dir := range expectedDirs {
		if !ExcludedDirs[dir] {
			t.Errorf("ExcludedDirs missing %q", dir)
		}
	}

	if len(ExcludedDirs) != len(expectedDirs) {
		t.Errorf("ExcludedDirs has %d entries, want %d", len(ExcludedDirs), len(expectedDirs))
	}
}

func TestDataDirWithDefault(t *testing.T) {
	// Save original XDG_DATA_HOME
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_DATA_HOME")
		}
	}()

	// Unset XDG_DATA_HOME to test default behavior
	os.Unsetenv("XDG_DATA_HOME")

	dataDir := DataDir()

	// Should use ~/.local/share/skillreg
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	expected := filepath.Join(home, ".local", "share", "skillreg")
	if dataDir != expected {
		t.Errorf("DataDir() = %q, want %q", dataDir, expected)
	}
}

func TestDataDirWithCustomXDG(t *testing.T) {
	// Save original XDG_DATA_HOME
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_DATA_HOME")
		}
	}()

	// Set custom XDG_DATA_HOME
	customXDG := "/tmp/test-xdg-data"
	os.Setenv("XDG_DATA_HOME", customXDG)

	dataDir := DataDir()

	expected := filepath.Join(customXDG, "skillreg")
	if dataDir != expected {
		t.Errorf("DataDir() = %q, want %q", dataDir, expected)
	}
}

func TestDBPath(t *testing.T) {
	// Save original XDG_DATA_HOME
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		if originalXDG != "" {
			os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			os.Unsetenv("XDG_DATA_HOME")
		}
	}()

	// Set custom XDG_DATA_HOME for predictable testing
	customXDG := "/tmp/test-xdg-data"
	os.Setenv("XDG_DATA_HOME", customXDG)

	dbPath := DBPath()

	expected := filepath.Join(customXDG, "skillreg", "skillreg.db")
	if dbPath != expected {
		t.Errorf("DBPath() = %q, want %q", dbPath, expected)
	}
}
