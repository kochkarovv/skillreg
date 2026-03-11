package models

import (
	"fmt"
	"time"

	"github.com/vladyslav/skillreg/internal/db"
)

// Skill represents a discovered skill file within a source repository.
type Skill struct {
	ID           int64
	SourceID     int64
	Name         string
	OriginalPath string
	Description  string
	DiscoveredAt time.Time
}

// CreateSkill inserts a new skill record and returns it.
func CreateSkill(d *db.Database, sourceID int64, name, originalPath, description string) (*Skill, error) {
	res, err := d.DB.Exec(
		`INSERT INTO skills (source_id, name, original_path, description) VALUES (?, ?, ?, ?)`,
		sourceID, name, originalPath, description,
	)
	if err != nil {
		return nil, fmt.Errorf("create skill: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("create skill last insert id: %w", err)
	}
	return GetSkill(d, id)
}

// GetSkill retrieves a skill by ID.
func GetSkill(d *db.Database, id int64) (*Skill, error) {
	row := d.DB.QueryRow(
		`SELECT id, source_id, name, original_path, description, discovered_at FROM skills WHERE id = ?`, id,
	)
	return scanSkill(row)
}

// ListSkillsBySource returns all skills for a given source.
func ListSkillsBySource(d *db.Database, sourceID int64) ([]*Skill, error) {
	rows, err := d.DB.Query(
		`SELECT id, source_id, name, original_path, description, discovered_at FROM skills WHERE source_id = ? ORDER BY name`,
		sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("list skills by source: %w", err)
	}
	defer rows.Close()
	return collectSkills(rows)
}

// ListAllSkills returns every skill across all sources.
func ListAllSkills(d *db.Database) ([]*Skill, error) {
	rows, err := d.DB.Query(
		`SELECT id, source_id, name, original_path, description, discovered_at FROM skills ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all skills: %w", err)
	}
	defer rows.Close()
	return collectSkills(rows)
}

// DeleteSkillsBySource removes all skills belonging to a source.
func DeleteSkillsBySource(d *db.Database, sourceID int64) error {
	_, err := d.DB.Exec(`DELETE FROM skills WHERE source_id = ?`, sourceID)
	if err != nil {
		return fmt.Errorf("delete skills by source: %w", err)
	}
	return nil
}

func collectSkills(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]*Skill, error) {
	var skills []*Skill
	for rows.Next() {
		sk, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		skills = append(skills, sk)
	}
	return skills, rows.Err()
}

func scanSkill(s scanner) (*Skill, error) {
	var sk Skill
	var discoveredAt string
	err := s.Scan(
		&sk.ID,
		&sk.SourceID,
		&sk.Name,
		&sk.OriginalPath,
		&sk.Description,
		&discoveredAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan skill: %w", err)
	}
	t, err := parseTime(discoveredAt)
	if err != nil {
		return nil, fmt.Errorf("parse skill discovered_at: %w", err)
	}
	sk.DiscoveredAt = t
	return &sk, nil
}
