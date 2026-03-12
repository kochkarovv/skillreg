package models

import (
	"fmt"
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

// Source represents a configured skill source repository.
type Source struct {
	ID            int64
	Name          string
	Path          string
	RemoteURL     string
	AutoUpdate    bool
	LastCheckedAt *time.Time
	CreatedAt     time.Time
}

// CreateSource inserts a new source record and returns the created Source.
func CreateSource(d *db.Database, name, path, remoteURL string) (*Source, error) {
	res, err := d.DB.Exec(
		`INSERT INTO sources (name, path, remote_url) VALUES (?, ?, ?)`,
		name, path, remoteURL,
	)
	if err != nil {
		return nil, fmt.Errorf("create source: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create source last insert id: %w", err)
	}
	return GetSource(d, id)
}

// GetSource retrieves a source by its ID.
func GetSource(d *db.Database, id int64) (*Source, error) {
	row := d.DB.QueryRow(
		`SELECT id, name, path, remote_url, auto_update, last_checked_at, created_at FROM sources WHERE id = ?`,
		id,
	)
	return scanSource(row)
}

// ListSources returns all sources ordered by created_at.
func ListSources(d *db.Database) ([]*Source, error) {
	rows, err := d.DB.Query(
		`SELECT id, name, path, remote_url, auto_update, last_checked_at, created_at FROM sources ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// DeleteSource removes a source by ID.
func DeleteSource(d *db.Database, id int64) error {
	_, err := d.DB.Exec(`DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return nil
}

// SetSourceAutoUpdate toggles the auto_update flag for a source.
func SetSourceAutoUpdate(d *db.Database, id int64, autoUpdate bool) error {
	_, err := d.DB.Exec(`UPDATE sources SET auto_update = ? WHERE id = ?`, autoUpdate, id)
	if err != nil {
		return fmt.Errorf("set source auto_update: %w", err)
	}
	return nil
}

// UpdateSourceLastChecked sets last_checked_at to the current UTC time.
func UpdateSourceLastChecked(d *db.Database, id int64) error {
	_, err := d.DB.Exec(
		`UPDATE sources SET last_checked_at = CURRENT_TIMESTAMP WHERE id = ?`, id,
	)
	if err != nil {
		return fmt.Errorf("update source last_checked_at: %w", err)
	}
	return nil
}

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanSource(s rowScanner) (*Source, error) {
	var src Source
	var lastChecked *string
	var createdAt string
	err := s.Scan(
		&src.ID,
		&src.Name,
		&src.Path,
		&src.RemoteURL,
		&src.AutoUpdate,
		&lastChecked,
		&createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	if lastChecked != nil {
		t, err := parseTime(*lastChecked)
		if err != nil {
			return nil, fmt.Errorf("parse last_checked_at: %w", err)
		}
		src.LastCheckedAt = &t
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	src.CreatedAt = t
	return &src, nil
}

// parseTime handles the common SQLite datetime formats.
func parseTime(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}
