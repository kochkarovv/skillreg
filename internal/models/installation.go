package models

import (
	"fmt"
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

// Installation represents a skill that has been symlinked into a specific provider instance.
type Installation struct {
	ID            int64
	SkillID       int64
	InstanceID    int64
	SymlinkPath   string
	InstalledName string
	InstalledAt   time.Time
	Status        string
}

// CreateInstallation inserts a new installation record and returns it.
func CreateInstallation(d *db.Database, skillID, instanceID int64, symlinkPath, installedName string) (*Installation, error) {
	res, err := d.DB.Exec(
		`INSERT INTO installations (skill_id, instance_id, symlink_path, installed_name) VALUES (?, ?, ?, ?)`,
		skillID, instanceID, symlinkPath, installedName,
	)
	if err != nil {
		return nil, fmt.Errorf("create installation: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create installation last insert id: %w", err)
	}
	return GetInstallation(d, id)
}

// GetInstallation retrieves an installation by ID.
func GetInstallation(d *db.Database, id int64) (*Installation, error) {
	row := d.DB.QueryRow(
		`SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations WHERE id = ?`, id,
	)
	return scanInstallation(row)
}

// ListInstallationsByInstance returns all installations for a given instance.
func ListInstallationsByInstance(d *db.Database, instanceID int64) ([]*Installation, error) {
	rows, err := d.DB.Query(
		`SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations WHERE instance_id = ? ORDER BY installed_at`,
		instanceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list installations by instance: %w", err)
	}
	defer rows.Close()
	return collectInstallations(rows)
}

// ListInstallationsBySkill returns all installations for a given skill.
func ListInstallationsBySkill(d *db.Database, skillID int64) ([]*Installation, error) {
	rows, err := d.DB.Query(
		`SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations WHERE skill_id = ? ORDER BY installed_at`,
		skillID,
	)
	if err != nil {
		return nil, fmt.Errorf("list installations by skill: %w", err)
	}
	defer rows.Close()
	return collectInstallations(rows)
}

// ListAllInstallations returns every installation record.
func ListAllInstallations(d *db.Database) ([]*Installation, error) {
	rows, err := d.DB.Query(
		`SELECT id, skill_id, instance_id, symlink_path, installed_name, installed_at, status FROM installations ORDER BY installed_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all installations: %w", err)
	}
	defer rows.Close()
	return collectInstallations(rows)
}

// UpdateInstallationStatus changes the status field of an installation.
func UpdateInstallationStatus(d *db.Database, id int64, status string) error {
	_, err := d.DB.Exec(`UPDATE installations SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update installation status: %w", err)
	}
	return nil
}

// DeleteInstallation removes an installation by ID.
func DeleteInstallation(d *db.Database, id int64) error {
	_, err := d.DB.Exec(`DELETE FROM installations WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete installation: %w", err)
	}
	return nil
}

func collectInstallations(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*Installation, error) {
	var installations []*Installation
	for rows.Next() {
		inst, err := scanInstallation(rows)
		if err != nil {
			return nil, err
		}
		installations = append(installations, inst)
	}
	return installations, rows.Err()
}

func scanInstallation(s rowScanner) (*Installation, error) {
	var inst Installation
	var installedAt string
	err := s.Scan(
		&inst.ID,
		&inst.SkillID,
		&inst.InstanceID,
		&inst.SymlinkPath,
		&inst.InstalledName,
		&installedAt,
		&inst.Status,
	)
	if err != nil {
		return nil, fmt.Errorf("scan installation: %w", err)
	}
	t, err := parseTime(installedAt)
	if err != nil {
		return nil, fmt.Errorf("parse installation installed_at: %w", err)
	}
	inst.InstalledAt = t
	return &inst, nil
}
