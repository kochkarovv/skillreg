package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Database represents a connection to the SQLite database.
type Database struct {
	DB *sql.DB
}

// Open creates the parent directory if needed, opens a SQLite database connection
// with WAL mode and foreign keys enabled, runs migrations, and returns a Database.
func Open(path string) (*Database, error) {
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dir, err)
	}

	// Open SQLite database with WAL mode and foreign keys enabled
	connStr := path + "?_pragma=journal_mode(wal)&_pragma=foreign_keys(on)"
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", path, err)
	}

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &Database{DB: sqlDB}

	// Run migrations
	if err := db.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	if d.DB != nil {
		return d.DB.Close()
	}
	return nil
}

// migrate runs all migration SQL statements to create tables.
func (d *Database) migrate() error {
	for i, migration := range migrations {
		if _, err := d.DB.Exec(migration); err != nil {
			return fmt.Errorf("migration %d failed: %w", i, err)
		}
	}
	return nil
}

// SeedProviders inserts all default providers into the database.
// Uses INSERT OR IGNORE for new providers and updates config_dir_prefix
// for existing builtin providers to keep them in sync with code changes.
func (d *Database) SeedProviders() error {
	for _, provider := range DefaultProviders {
		_, err := d.DB.Exec(
			"INSERT OR IGNORE INTO providers (name, config_dir_prefix, is_builtin) VALUES (?, ?, ?)",
			provider.Name,
			provider.ConfigDirPrefix,
			1, // is_builtin = true
		)
		if err != nil {
			return fmt.Errorf("failed to insert provider %s: %w", provider.Name, err)
		}
		// Update config_dir_prefix for existing builtin providers
		_, err = d.DB.Exec(
			"UPDATE providers SET config_dir_prefix = ? WHERE name = ? AND is_builtin = 1",
			provider.ConfigDirPrefix,
			provider.Name,
		)
		if err != nil {
			return fmt.Errorf("failed to update provider %s: %w", provider.Name, err)
		}
	}
	return nil
}
