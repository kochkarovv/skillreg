package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/linker"
	"github.com/vladyslav/skillreg/internal/models"
	"github.com/vladyslav/skillreg/internal/scanner"
)

// TestFullWorkflow exercises the end-to-end workflow:
// DB → source → scan → skill → instance → symlink → health check → uninstall
func TestFullWorkflow(t *testing.T) {
	// Create temporary directories for DB and repo
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	repoDir := filepath.Join(tempDir, "test-repo")

	// Step 1: Open a temp DB and seed providers
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	defer d.Close()

	if err := d.SeedProviders(); err != nil {
		t.Fatalf("failed to seed providers: %v", err)
	}

	// Step 2: Create a fake git repo with 2 skills
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("failed to create repo directory: %v", err)
	}

	// Initialize git repo
	if err := initGitRepo(repoDir); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Create skill 1: brainstorm
	skill1Dir := filepath.Join(repoDir, "brainstorm")
	if err := os.MkdirAll(skill1Dir, 0755); err != nil {
		t.Fatalf("failed to create skill 1 directory: %v", err)
	}
	skill1Content := `---
description: Brainstorming skill for generating ideas
---

# Brainstorm Skill

This skill helps generate creative ideas.
`
	if err := os.WriteFile(filepath.Join(skill1Dir, "SKILL.md"), []byte(skill1Content), 0644); err != nil {
		t.Fatalf("failed to write skill 1 SKILL.md: %v", err)
	}

	// Create skill 2: debug
	skill2Dir := filepath.Join(repoDir, "debug")
	if err := os.MkdirAll(skill2Dir, 0755); err != nil {
		t.Fatalf("failed to create skill 2 directory: %v", err)
	}
	skill2Content := `---
description: Debugging skill for finding issues
---

# Debug Skill

This skill helps debug code.
`
	if err := os.WriteFile(filepath.Join(skill2Dir, "SKILL.md"), []byte(skill2Content), 0644); err != nil {
		t.Fatalf("failed to write skill 2 SKILL.md: %v", err)
	}

	// Step 3: Register the repo as a source
	source, err := models.CreateSource(d, "test-source", repoDir, "https://github.com/test/skills")
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	if source.ID == 0 {
		t.Fatal("created source has zero ID")
	}

	// Step 4: Scan the repo
	discoveredSkills, err := scanner.ScanRepo(repoDir)
	if err != nil {
		t.Fatalf("failed to scan repo: %v", err)
	}
	if len(discoveredSkills) != 2 {
		t.Fatalf("expected 2 discovered skills, got %d", len(discoveredSkills))
	}

	// Step 5: Store discovered skills
	var storedSkills []*models.Skill
	for _, discovered := range discoveredSkills {
		skill, err := models.CreateSkill(d, source.ID, discovered.Name, discovered.Path, discovered.Description)
		if err != nil {
			t.Fatalf("failed to create skill: %v", err)
		}
		storedSkills = append(storedSkills, skill)
	}

	if len(storedSkills) != 2 {
		t.Fatalf("expected 2 stored skills, got %d", len(storedSkills))
	}

	// Verify skills can be retrieved
	listedSkills, err := models.ListSkillsBySource(d, source.ID)
	if err != nil {
		t.Fatalf("failed to list skills by source: %v", err)
	}
	if len(listedSkills) != 2 {
		t.Fatalf("expected 2 listed skills, got %d", len(listedSkills))
	}

	// Step 6: Create an instance
	providers, err := models.ListProviders(d)
	if err != nil {
		t.Fatalf("failed to list providers: %v", err)
	}
	if len(providers) == 0 {
		t.Fatal("no providers available after seeding")
	}

	skillsPath := filepath.Join(tempDir, "skills")
	instance, err := models.CreateInstance(d, providers[0].ID, "test-instance", skillsPath, true)
	if err != nil {
		t.Fatalf("failed to create instance: %v", err)
	}
	if instance.ID == 0 {
		t.Fatal("created instance has zero ID")
	}

	// Step 7: Install first skill (create symlink + installation record)
	skillToInstall := storedSkills[0]
	symlinkPath := filepath.Join(skillsPath, skillToInstall.Name)

	// Create the symlink
	if err := linker.CreateSymlink(skillToInstall.OriginalPath, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Create the installation record
	installation, err := models.CreateInstallation(d, skillToInstall.ID, instance.ID, symlinkPath, skillToInstall.Name)
	if err != nil {
		t.Fatalf("failed to create installation: %v", err)
	}
	if installation.ID == 0 {
		t.Fatal("created installation has zero ID")
	}

	// Step 8: Verify symlink is active
	status := linker.CheckSymlink(symlinkPath, skillToInstall.OriginalPath)
	if status != linker.StatusActive {
		t.Fatalf("expected symlink status %q, got %q", linker.StatusActive, status)
	}

	// Verify the symlink points to the skill directory
	if !linker.IsSymlink(symlinkPath) {
		t.Fatal("expected symlink to exist at installation target")
	}

	// Verify installation can be retrieved
	retrievedInstallation, err := models.GetInstallation(d, installation.ID)
	if err != nil {
		t.Fatalf("failed to get installation: %v", err)
	}
	if retrievedInstallation.SkillID != skillToInstall.ID {
		t.Fatalf("expected installation skill ID %d, got %d", skillToInstall.ID, retrievedInstallation.SkillID)
	}

	// Step 9: Uninstall (remove symlink + delete installation record)
	if err := linker.RemoveSymlink(symlinkPath); err != nil {
		t.Fatalf("failed to remove symlink: %v", err)
	}

	if err := models.DeleteInstallation(d, installation.ID); err != nil {
		t.Fatalf("failed to delete installation: %v", err)
	}

	// Step 10: Verify symlink is now orphaned
	status = linker.CheckSymlink(symlinkPath, skillToInstall.OriginalPath)
	if status != linker.StatusOrphaned {
		t.Fatalf("expected symlink status %q after removal, got %q", linker.StatusOrphaned, status)
	}

	// Verify the symlink no longer exists
	if linker.IsSymlink(symlinkPath) {
		t.Fatal("expected symlink to be removed")
	}

	// Verify installation is deleted
	_, err = models.GetInstallation(d, installation.ID)
	if err == nil {
		t.Fatal("expected error when getting deleted installation")
	}

	// Verify no installations for the instance
	instanceInstallations, err := models.ListInstallationsByInstance(d, instance.ID)
	if err != nil {
		t.Fatalf("failed to list installations: %v", err)
	}
	if len(instanceInstallations) != 0 {
		t.Fatalf("expected 0 installations after uninstall, got %d", len(instanceInstallations))
	}
}

// initGitRepo initializes a git repository at the given path
func initGitRepo(repoPath string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
