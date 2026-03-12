package models

import (
	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/scanner"
)

// SyncResult holds the outcome of a source sync.
type SyncResult struct {
	Added   int
	Updated int
	Removed int
}

// SyncSourceSkills rescans a source repository and synchronises the database:
//   - New skills (found on disk but not in DB) are added
//   - Moved/updated skills (name exists in DB but path or description changed) are updated
//   - Stale skills (in DB but not found on disk) are removed along with their installations
func SyncSourceSkills(d *db.Database, source *Source) (*SyncResult, error) {
	discovered, err := scanner.ScanRepo(source.Path)
	if err != nil {
		return nil, err
	}

	existing, err := ListSkillsBySource(d, source.ID)
	if err != nil {
		return nil, err
	}

	// Index existing skills by name
	byName := make(map[string]*Skill, len(existing))
	for _, sk := range existing {
		byName[sk.Name] = sk
	}

	// Index discovered skills by name
	discoveredByName := make(map[string]scanner.DiscoveredSkill, len(discovered))
	for _, ds := range discovered {
		discoveredByName[ds.Name] = ds
	}

	var result SyncResult

	// Add or update
	for _, ds := range discovered {
		if sk, ok := byName[ds.Name]; ok {
			// Exists — update if path or description changed
			if sk.OriginalPath != ds.Path || sk.Description != ds.Description {
				if err := UpdateSkill(d, sk.ID, ds.Path, ds.Description); err != nil {
					return nil, err
				}
				result.Updated++
			}
		} else {
			// New skill
			_, err := CreateSkill(d, source.ID, ds.Name, ds.Path, ds.Description)
			if err != nil {
				return nil, err
			}
			result.Added++
		}
	}

	// Remove stale skills
	for _, sk := range existing {
		if _, ok := discoveredByName[sk.Name]; !ok {
			// Also clean up installations for this skill
			installations, _ := ListInstallationsBySkill(d, sk.ID)
			for _, inst := range installations {
				_ = DeleteInstallation(d, inst.ID)
			}
			if err := DeleteSkill(d, sk.ID); err != nil {
				return nil, err
			}
			result.Removed++
		}
	}

	return &result, nil
}

// SyncAllSources runs SyncSourceSkills for every registered source.
func SyncAllSources(d *db.Database) error {
	sources, err := ListSources(d)
	if err != nil {
		return err
	}
	for _, src := range sources {
		if _, err := SyncSourceSkills(d, src); err != nil {
			// Continue syncing other sources even if one fails
			continue
		}
	}
	return nil
}
