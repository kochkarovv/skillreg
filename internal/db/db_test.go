package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesDBFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Ensure the file doesn't exist before opening
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatalf("DB file already exists at %s", dbPath)
	}

	// Open the database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	// Verify the file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Errorf("DB file was not created at %s: %v", dbPath, err)
	}
}

func TestOpenRunsMigrations(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open the database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	// Verify all 5 tables were created by querying sqlite_master
	expectedTables := []string{"sources", "providers", "instances", "skills", "installations"}

	for _, tableName := range expectedTables {
		var count int
		err := db.DB.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			tableName,
		).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query sqlite_master for table %s: %v", tableName, err)
		}
		if count != 1 {
			t.Errorf("Table %s does not exist in database", tableName)
		}
	}
}

func TestSeedProvidersCreatesAllProviders(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	// Seed providers
	err = db.SeedProviders()
	if err != nil {
		t.Fatalf("SeedProviders() failed: %v", err)
	}

	// Verify all 6 default providers were inserted
	var count int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM providers").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count providers: %v", err)
	}
	if count != len(DefaultProviders) {
		t.Errorf("Expected %d providers, got %d", len(DefaultProviders), count)
	}

	// Verify each provider exists
	for _, provider := range DefaultProviders {
		var id int
		var name string
		var configDir string
		err := db.DB.QueryRow(
			"SELECT id, name, config_dir_prefix FROM providers WHERE name = ?",
			provider.Name,
		).Scan(&id, &name, &configDir)
		if err != nil {
			t.Errorf("Provider %s not found in database: %v", provider.Name, err)
		}
		if configDir != provider.ConfigDirPrefix {
			t.Errorf("Provider %s has config_dir_prefix %q, expected %q", name, configDir, provider.ConfigDirPrefix)
		}
	}
}

func TestSeedProvidersIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer db.Close()

	// Seed providers once
	err = db.SeedProviders()
	if err != nil {
		t.Fatalf("First SeedProviders() failed: %v", err)
	}

	// Count providers after first seed
	var countAfterFirst int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM providers").Scan(&countAfterFirst)
	if err != nil {
		t.Fatalf("Failed to count providers after first seed: %v", err)
	}

	// Seed providers again
	err = db.SeedProviders()
	if err != nil {
		t.Fatalf("Second SeedProviders() failed: %v", err)
	}

	// Count providers after second seed
	var countAfterSecond int
	err = db.DB.QueryRow("SELECT COUNT(*) FROM providers").Scan(&countAfterSecond)
	if err != nil {
		t.Fatalf("Failed to count providers after second seed: %v", err)
	}

	// Verify counts are the same (idempotent)
	if countAfterFirst != countAfterSecond {
		t.Errorf("SeedProviders() is not idempotent: %d after first, %d after second", countAfterFirst, countAfterSecond)
	}
	if countAfterSecond != len(DefaultProviders) {
		t.Errorf("Expected %d providers after idempotent call, got %d", len(DefaultProviders), countAfterSecond)
	}
}
