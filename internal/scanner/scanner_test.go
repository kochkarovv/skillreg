package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDescriptionWithYAMLFrontmatter(t *testing.T) {
	content := `---
name: Test Skill
description: This is the skill description
version: 1.0
---

# Some heading

This is other content.`

	result := ParseDescription(content)
	expected := "This is the skill description"
	if result != expected {
		t.Errorf("ParseDescription() = %q, want %q", result, expected)
	}
}

func TestParseDescriptionWithoutFrontmatter(t *testing.T) {
	content := `# Heading

This is the first non-heading line.
This is another line.`

	result := ParseDescription(content)
	expected := "This is the first non-heading line."
	if result != expected {
		t.Errorf("ParseDescription() = %q, want %q", result, expected)
	}
}

func TestParseDescriptionEmptyContent(t *testing.T) {
	result := ParseDescription("")
	expected := ""
	if result != expected {
		t.Errorf("ParseDescription() = %q, want %q", result, expected)
	}
}

func TestParseDescriptionOnlyHeadings(t *testing.T) {
	content := `# Heading
## Subheading
### Sub-subheading`

	result := ParseDescription(content)
	expected := ""
	if result != expected {
		t.Errorf("ParseDescription() = %q, want %q", result, expected)
	}
}

func TestParseDescriptionWithEmptyLines(t *testing.T) {
	content := `# Heading


This is the description after empty lines.`

	result := ParseDescription(content)
	expected := "This is the description after empty lines."
	if result != expected {
		t.Errorf("ParseDescription() = %q, want %q", result, expected)
	}
}

func TestScanRepoTopLevel(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "skillreg-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a top-level skill
	skillDir := filepath.Join(tmpDir, "my_skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill dir: %v", err)
	}

	skillFile := filepath.Join(skillDir, "SKILL.md")
	content := `---
description: My test skill
---

# My Skill

This is a test skill.`

	if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Scan the repo
	skills, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("ScanRepo() returned %d skills, want 1", len(skills))
	}

	if len(skills) > 0 {
		if skills[0].Name != "my_skill" {
			t.Errorf("Skill name = %q, want %q", skills[0].Name, "my_skill")
		}
		if skills[0].Description != "My test skill" {
			t.Errorf("Skill description = %q, want %q", skills[0].Description, "My test skill")
		}
		if !strings.Contains(skills[0].Path, "my_skill") {
			t.Errorf("Skill path = %q, should contain %q", skills[0].Path, "my_skill")
		}
	}
}

func TestScanRepoNestedSkills(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skillreg-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested skills
	skillDirs := []string{
		filepath.Join(tmpDir, "skills", "brainstorming"),
		filepath.Join(tmpDir, ".claude", "skills", "debug"),
	}

	for i, skillDir := range skillDirs {
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("Failed to create skill dir: %v", err)
		}

		skillFile := filepath.Join(skillDir, "SKILL.md")
		content := `---
description: Test skill ` + string(rune(i+1)) + `
---

# Skill ` + string(rune(i+1))

		if err := os.WriteFile(skillFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write SKILL.md: %v", err)
		}
	}

	skills, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(skills) != 2 {
		t.Errorf("ScanRepo() returned %d skills, want 2", len(skills))
	}

	// Verify skill names are correct
	skillNames := make(map[string]bool)
	for _, skill := range skills {
		skillNames[skill.Name] = true
	}

	expectedNames := []string{"brainstorming", "debug"}
	for _, name := range expectedNames {
		if !skillNames[name] {
			t.Errorf("Skill %q not found in results", name)
		}
	}
}

func TestScanRepoExcludesGitDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skillreg-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a skill in .git (should be excluded)
	gitSkillDir := filepath.Join(tmpDir, ".git", "objects", "skill_in_git")
	if err := os.MkdirAll(gitSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create git skill dir: %v", err)
	}

	skillFile := filepath.Join(gitSkillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("description: Should not find this"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create a regular skill
	regularSkillDir := filepath.Join(tmpDir, "regular_skill")
	if err := os.MkdirAll(regularSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create regular skill dir: %v", err)
	}

	skillFile = filepath.Join(regularSkillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("description: Regular skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	skills, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("ScanRepo() returned %d skills, want 1 (excluding .git)", len(skills))
	}

	if len(skills) > 0 && skills[0].Name != "regular_skill" {
		t.Errorf("Found skill %q, want regular_skill", skills[0].Name)
	}
}

func TestScanRepoExcludesNodeModules(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skillreg-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a skill in node_modules (should be excluded)
	nodeSkillDir := filepath.Join(tmpDir, "node_modules", "some_package", "skill")
	if err := os.MkdirAll(nodeSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create node_modules skill dir: %v", err)
	}

	skillFile := filepath.Join(nodeSkillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("description: Should not find this"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create a regular skill
	regularSkillDir := filepath.Join(tmpDir, "my_skill")
	if err := os.MkdirAll(regularSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create regular skill dir: %v", err)
	}

	skillFile = filepath.Join(regularSkillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("description: Regular skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	skills, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("ScanRepo() returned %d skills, want 1 (excluding node_modules)", len(skills))
	}

	if len(skills) > 0 && skills[0].Name != "my_skill" {
		t.Errorf("Found skill %q, want my_skill", skills[0].Name)
	}
}

func TestScanRepoIncludesDotDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skillreg-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a skill in .claude directory (should be included)
	claudeSkillDir := filepath.Join(tmpDir, ".claude", "skills", "my_skill")
	if err := os.MkdirAll(claudeSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create .claude skill dir: %v", err)
	}

	skillFile := filepath.Join(claudeSkillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("description: Skill in .claude"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	skills, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("ScanRepo() returned %d skills, want 1", len(skills))
	}

	if len(skills) > 0 && skills[0].Name != "my_skill" {
		t.Errorf("Found skill %q, want my_skill", skills[0].Name)
	}
}

func TestScanRepoMultipleExcludedDirs(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skillreg-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create skills in various excluded directories
	excludedDirs := []string{"node_modules", "vendor", "__pycache__", ".venv"}
	for i, excluded := range excludedDirs {
		skillDir := filepath.Join(tmpDir, excluded, "skill_excluded_"+string(rune(48+i)))
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatalf("Failed to create %s skill dir: %v", excluded, err)
		}

		skillFile := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillFile, []byte("description: Should exclude"), 0644); err != nil {
			t.Fatalf("Failed to write SKILL.md: %v", err)
		}
	}

	// Create a regular skill
	regularSkillDir := filepath.Join(tmpDir, "visible_skill")
	if err := os.MkdirAll(regularSkillDir, 0755); err != nil {
		t.Fatalf("Failed to create regular skill dir: %v", err)
	}

	skillFile := filepath.Join(regularSkillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("description: Visible skill"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	skills, err := ScanRepo(tmpDir)
	if err != nil {
		t.Fatalf("ScanRepo failed: %v", err)
	}

	if len(skills) != 1 {
		t.Errorf("ScanRepo() returned %d skills, want 1", len(skills))
	}

	if len(skills) > 0 && skills[0].Name != "visible_skill" {
		t.Errorf("Found skill %q, want visible_skill", skills[0].Name)
	}
}

func TestScanRepoInvalidPath(t *testing.T) {
	_, err := ScanRepo("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Errorf("ScanRepo() should return error for invalid path")
	}
}

func TestDiscoveredSkillStruct(t *testing.T) {
	skill := DiscoveredSkill{
		Name:        "test_skill",
		Path:        "/path/to/test_skill",
		Description: "A test skill",
	}

	if skill.Name != "test_skill" {
		t.Errorf("DiscoveredSkill.Name = %q, want %q", skill.Name, "test_skill")
	}

	if skill.Path != "/path/to/test_skill" {
		t.Errorf("DiscoveredSkill.Path = %q, want %q", skill.Path, "/path/to/test_skill")
	}

	if skill.Description != "A test skill" {
		t.Errorf("DiscoveredSkill.Description = %q, want %q", skill.Description, "A test skill")
	}
}
