package models

import (
	"fmt"
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

// Instance represents a single installation of a provider on the local machine.
type Instance struct {
	ID               int64
	ProviderID       int64
	Name             string
	GlobalSkillsPath string
	IsDefault        bool
	CreatedAt        time.Time
}

// CreateInstance inserts a new instance and returns it.
func CreateInstance(d *db.Database, providerID int64, name, globalSkillsPath string, isDefault bool) (*Instance, error) {
	res, err := d.DB.Exec(
		`INSERT INTO instances (provider_id, name, global_skills_path, is_default) VALUES (?, ?, ?, ?)`,
		providerID, name, globalSkillsPath, isDefault,
	)
	if err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create instance last insert id: %w", err)
	}
	return GetInstance(d, id)
}

// GetInstance retrieves an instance by ID.
func GetInstance(d *db.Database, id int64) (*Instance, error) {
	row := d.DB.QueryRow(
		`SELECT id, provider_id, name, global_skills_path, is_default, created_at FROM instances WHERE id = ?`, id,
	)
	return scanInstance(row)
}

// ListInstancesByProvider returns all instances for a given provider.
func ListInstancesByProvider(d *db.Database, providerID int64) ([]*Instance, error) {
	rows, err := d.DB.Query(
		`SELECT id, provider_id, name, global_skills_path, is_default, created_at FROM instances WHERE provider_id = ? ORDER BY created_at`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("list instances by provider: %w", err)
	}
	defer rows.Close()
	return collectInstances(rows)
}

// ListAllInstances returns every instance across all providers.
func ListAllInstances(d *db.Database) ([]*Instance, error) {
	rows, err := d.DB.Query(
		`SELECT id, provider_id, name, global_skills_path, is_default, created_at FROM instances ORDER BY created_at`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all instances: %w", err)
	}
	defer rows.Close()
	return collectInstances(rows)
}

// DeleteInstance removes an instance by ID.
func DeleteInstance(d *db.Database, id int64) error {
	_, err := d.DB.Exec(`DELETE FROM instances WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete instance: %w", err)
	}
	return nil
}

func collectInstances(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*Instance, error) {
	var instances []*Instance
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

func scanInstance(s scanner) (*Instance, error) {
	var inst Instance
	var createdAt string
	err := s.Scan(
		&inst.ID,
		&inst.ProviderID,
		&inst.Name,
		&inst.GlobalSkillsPath,
		&inst.IsDefault,
		&createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan instance: %w", err)
	}
	t, err := parseTime(createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse instance created_at: %w", err)
	}
	inst.CreatedAt = t
	return &inst, nil
}
