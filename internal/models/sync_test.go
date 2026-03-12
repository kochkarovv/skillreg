package models_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vladyslav/skillreg/internal/models"
)

// createSkillDir creates a directory with a SKILL.md file.
func createSkillDir(t *testing.T, dir, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	content := "---\ndescription: " + description + "\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile SKILL.md in %s: %v", dir, err)
	}
}

func TestSyncSourceSkills_NewSkillsAdded(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	createSkillDir(t, filepath.Join(root, "skill-a"), "Skill A")

	src, err := models.CreateSource(d, "test-src", root, "")
	if err != nil {
		t.Fatal(err)
	}
	// Manually add skill-a to DB
	models.CreateSkill(d, src.ID, "skill-a", filepath.Join(root, "skill-a"), "Skill A")

	// Now add a new skill on disk
	createSkillDir(t, filepath.Join(root, "skill-b"), "Skill B")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Errorf("Added = %d, want 1", result.Added)
	}
	if result.Updated != 0 {
		t.Errorf("Updated = %d, want 0", result.Updated)
	}
	if result.Removed != 0 {
		t.Errorf("Removed = %d, want 0", result.Removed)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 2 {
		t.Errorf("got %d skills, want 2", len(skills))
	}
}

func TestSyncSourceSkills_StaleSkillsRemoved(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	createSkillDir(t, filepath.Join(root, "skill-a"), "Skill A")

	src, _ := models.CreateSource(d, "test-src", root, "")
	models.CreateSkill(d, src.ID, "skill-a", filepath.Join(root, "skill-a"), "Skill A")
	models.CreateSkill(d, src.ID, "skill-gone", filepath.Join(root, "skill-gone"), "Gone")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1", result.Removed)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 1 {
		t.Errorf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "skill-a" {
		t.Errorf("remaining skill = %q, want skill-a", skills[0].Name)
	}
}

func TestSyncSourceSkills_PathUpdated(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	// Skill on disk is at core/skills/context-files
	createSkillDir(t, filepath.Join(root, "core", "skills", "context-files"), "Context files")

	src, _ := models.CreateSource(d, "test-src", root, "")
	// But DB has old path .github/skills/context-files
	models.CreateSkill(d, src.ID, "context-files", filepath.Join(root, ".github", "skills", "context-files"), "Context files")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1 {
		t.Errorf("Updated = %d, want 1", result.Updated)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 1 {
		t.Fatal("expected 1 skill")
	}
	expected := filepath.Join(root, "core", "skills", "context-files")
	if skills[0].OriginalPath != expected {
		t.Errorf("OriginalPath = %q, want %q", skills[0].OriginalPath, expected)
	}
}

func TestSyncSourceSkills_DescriptionUpdated(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	createSkillDir(t, filepath.Join(root, "my-skill"), "New description")

	src, _ := models.CreateSource(d, "test-src", root, "")
	models.CreateSkill(d, src.ID, "my-skill", filepath.Join(root, "my-skill"), "Old description")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Updated != 1 {
		t.Errorf("Updated = %d, want 1", result.Updated)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if skills[0].Description != "New description" {
		t.Errorf("Description = %q, want %q", skills[0].Description, "New description")
	}
}

func TestSyncSourceSkills_NoChanges(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	createSkillDir(t, filepath.Join(root, "skill-a"), "Skill A")

	src, _ := models.CreateSource(d, "test-src", root, "")
	models.CreateSkill(d, src.ID, "skill-a", filepath.Join(root, "skill-a"), "Skill A")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || result.Updated != 0 || result.Removed != 0 {
		t.Errorf("expected no changes, got Added=%d Updated=%d Removed=%d",
			result.Added, result.Updated, result.Removed)
	}
}

func TestSyncSourceSkills_CompleteRestructure(t *testing.T) {
	// Simulates the dev-copilot scenario: skills moved from .github/skills to core/skills
	d := newTestDB(t)
	root := t.TempDir()

	// Current disk layout: skills in core/skills and .claude/skills
	createSkillDir(t, filepath.Join(root, "core", "skills", "context-files"), "Context files")
	createSkillDir(t, filepath.Join(root, "core", "skills", "creating-pr"), "Creating PR")
	createSkillDir(t, filepath.Join(root, "core", "skills", "git-commit"), "Git commit")
	createSkillDir(t, filepath.Join(root, ".claude", "skills", "init-claude"), "Claude init")
	createSkillDir(t, filepath.Join(root, ".github", "skills", "init-copilot"), "Copilot init")

	src, _ := models.CreateSource(d, "dev-copilot", root, "")

	// DB has old paths (skills were previously in .github/skills)
	models.CreateSkill(d, src.ID, "context-files", filepath.Join(root, ".github", "skills", "context-files"), "Context files")
	models.CreateSkill(d, src.ID, "creating-pr", filepath.Join(root, ".github", "skills", "creating-pr"), "Creating PR")
	models.CreateSkill(d, src.ID, "git-commit", filepath.Join(root, ".github", "skills", "git-commit"), "Git commit")
	models.CreateSkill(d, src.ID, "init-copilot", filepath.Join(root, ".github", "skills", "init-copilot"), "Copilot init")
	// init-claude was previously in .claude/skills (and is still there)
	models.CreateSkill(d, src.ID, "init-claude", filepath.Join(root, ".claude", "skills", "init-claude"), "Claude init")
	// An old skill that was removed from disk
	models.CreateSkill(d, src.ID, "old-removed-skill", filepath.Join(root, ".github", "skills", "old-removed-skill"), "Gone")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}

	// 3 skills moved (context-files, creating-pr, git-commit)
	if result.Updated != 3 {
		t.Errorf("Updated = %d, want 3", result.Updated)
	}
	// 1 skill removed (old-removed-skill)
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1", result.Removed)
	}
	// 0 added (all existing skills match by name)
	if result.Added != 0 {
		t.Errorf("Added = %d, want 0", result.Added)
	}

	// Verify paths are correct now
	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 5 {
		t.Fatalf("got %d skills, want 5", len(skills))
	}

	for _, sk := range skills {
		if sk.Name == "context-files" && sk.OriginalPath != filepath.Join(root, "core", "skills", "context-files") {
			t.Errorf("context-files path = %q", sk.OriginalPath)
		}
		if sk.Name == "init-copilot" && sk.OriginalPath != filepath.Join(root, ".github", "skills", "init-copilot") {
			t.Errorf("init-copilot path = %q (should be unchanged)", sk.OriginalPath)
		}
	}
}

func TestSyncSourceSkills_RemoveCleansInstallations(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	// Only skill-a on disk
	createSkillDir(t, filepath.Join(root, "skill-a"), "Skill A")

	src, _ := models.CreateSource(d, "test-src", root, "")
	skA, _ := models.CreateSkill(d, src.ID, "skill-a", filepath.Join(root, "skill-a"), "A")
	skGone, _ := models.CreateSkill(d, src.ID, "skill-gone", filepath.Join(root, "skill-gone"), "Gone")

	// Create a provider and instance for installations
	prov, _ := models.CreateProvider(d, "test-provider", ".test")
	inst, _ := models.CreateInstance(d, prov.ID, "test-instance", "/tmp/test-skills", false)

	// Install both skills
	models.CreateInstallation(d, skA.ID, inst.ID, "/tmp/test-skills/skill-a", "skill-a")
	models.CreateInstallation(d, skGone.ID, inst.ID, "/tmp/test-skills/skill-gone", "skill-gone")

	// Verify 2 installations exist
	allInst, _ := models.ListAllInstallations(d)
	if len(allInst) != 2 {
		t.Fatalf("expected 2 installations before sync, got %d", len(allInst))
	}

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1", result.Removed)
	}

	// Installation for skill-gone should be cleaned up
	allInst, _ = models.ListAllInstallations(d)
	if len(allInst) != 1 {
		t.Errorf("expected 1 installation after sync, got %d", len(allInst))
	}
}

func TestSyncAllSources(t *testing.T) {
	d := newTestDB(t)

	root1 := t.TempDir()
	root2 := t.TempDir()

	createSkillDir(t, filepath.Join(root1, "skill-1"), "Source 1 skill")
	createSkillDir(t, filepath.Join(root2, "skill-2"), "Source 2 skill")

	src1, _ := models.CreateSource(d, "src1", root1, "")
	src2, _ := models.CreateSource(d, "src2", root2, "")

	// DB has stale skills for both
	models.CreateSkill(d, src1.ID, "stale-1", "/old/stale-1", "Stale")
	models.CreateSkill(d, src2.ID, "stale-2", "/old/stale-2", "Stale")

	err := models.SyncAllSources(d)
	if err != nil {
		t.Fatal(err)
	}

	skills, _ := models.ListAllSkills(d)
	// Should have skill-1 and skill-2 (stale ones removed, new ones added)
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}
	if !names["skill-1"] || !names["skill-2"] {
		t.Errorf("expected skill-1 and skill-2, got %v", names)
	}
}

func TestSyncSourceSkills_EmptySource(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	src, _ := models.CreateSource(d, "empty", root, "")
	models.CreateSkill(d, src.ID, "old-skill", filepath.Join(root, "old-skill"), "Old")

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Removed != 1 {
		t.Errorf("Removed = %d, want 1", result.Removed)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestSyncSourceSkills_FreshSource(t *testing.T) {
	d := newTestDB(t)
	root := t.TempDir()

	createSkillDir(t, filepath.Join(root, "new-a"), "A")
	createSkillDir(t, filepath.Join(root, "new-b"), "B")
	createSkillDir(t, filepath.Join(root, "core", "skills", "new-c"), "C")

	src, _ := models.CreateSource(d, "fresh", root, "")
	// No skills in DB yet

	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 3 {
		t.Errorf("Added = %d, want 3", result.Added)
	}

	skills, _ := models.ListSkillsBySource(d, src.ID)
	if len(skills) != 3 {
		t.Errorf("got %d skills, want 3", len(skills))
	}
}

func TestSyncSourceSkills_DuplicateNames(t *testing.T) {
	// If the scanner finds two skills with the same name (different paths),
	// the sync should handle the first one matched by name and add new ones.
	d := newTestDB(t)
	root := t.TempDir()

	// Two "init" skills at different paths
	createSkillDir(t, filepath.Join(root, ".claude", "skills", "init"), "Claude init")
	createSkillDir(t, filepath.Join(root, ".github", "skills", "init"), "GitHub init")

	src, _ := models.CreateSource(d, "dup-src", root, "")

	// The scanner will return both. SyncSourceSkills matches by name,
	// so it'll update one and leave the duplicate as "new".
	result, err := models.SyncSourceSkills(d, src)
	if err != nil {
		t.Fatal(err)
	}
	// Both should be added since DB is empty
	if result.Added != 2 {
		t.Errorf("Added = %d, want 2", result.Added)
	}
}
