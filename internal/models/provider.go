package models

import (
	"fmt"

	"github.com/vladyslav/skillreg/internal/db"
)

// Provider represents a supported AI tool that consumes skills (e.g. Claude, Codex).
type Provider struct {
	ID              int64
	Name            string
	ConfigDirPrefix string
	IsBuiltin       bool
}

// ListProviders returns all providers ordered by name.
func ListProviders(d *db.Database) ([]*Provider, error) {
	rows, err := d.DB.Query(
		`SELECT id, name, config_dir_prefix, is_builtin FROM providers ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []*Provider
	for rows.Next() {
		p, err := scanProvider(rows)
		if err != nil {
			return nil, err
		}
		providers = append(providers, p)
	}
	return providers, rows.Err()
}

// GetProvider retrieves a provider by ID.
func GetProvider(d *db.Database, id int64) (*Provider, error) {
	row := d.DB.QueryRow(
		`SELECT id, name, config_dir_prefix, is_builtin FROM providers WHERE id = ?`, id,
	)
	return scanProvider(row)
}

// CreateProvider inserts a new non-builtin provider and returns it.
func CreateProvider(d *db.Database, name, configDirPrefix string) (*Provider, error) {
	res, err := d.DB.Exec(
		`INSERT INTO providers (name, config_dir_prefix, is_builtin) VALUES (?, ?, 0)`,
		name, configDirPrefix,
	)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create provider last insert id: %w", err)
	}
	return GetProvider(d, id)
}

func scanProvider(s scanner) (*Provider, error) {
	var p Provider
	err := s.Scan(&p.ID, &p.Name, &p.ConfigDirPrefix, &p.IsBuiltin)
	if err != nil {
		return nil, fmt.Errorf("scan provider: %w", err)
	}
	return &p, nil
}
