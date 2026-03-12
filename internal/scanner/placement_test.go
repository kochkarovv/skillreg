package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// helper to create a SKILL.md at a given directory path
func createSkill(t *testing.T, dir, description string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	content := "---\ndescription: " + description + "\n---\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile SKILL.md in %s: %v", dir, err)
	}
}

func skillNames(skills []DiscoveredSkill) []string {
	names := make([]string, len(skills))
	for i, s := range skills {
		names[i] = s.Name
	}
	sort.Strings(names)
	return names
}

func skillByName(skills []DiscoveredSkill, name string) *DiscoveredSkill {
	for i := range skills {
		if skills[i].Name == name {
			return &skills[i]
		}
	}
	return nil
}

// --- Placement combination tests ---

func TestPlacement_TopLevel(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "skill-a"), "Top level A")
	createSkill(t, filepath.Join(root, "skill-b"), "Top level B")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}

	a := skillByName(skills, "skill-a")
	if a == nil || a.Path != filepath.Join(root, "skill-a") {
		t.Errorf("skill-a path = %v, want %s", a, filepath.Join(root, "skill-a"))
	}
}

func TestPlacement_NestedInCoreSkills(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "core", "skills", "my-skill"), "Nested in core/skills")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("name = %q, want my-skill", skills[0].Name)
	}
	if skills[0].Path != filepath.Join(root, "core", "skills", "my-skill") {
		t.Errorf("path = %q, want core/skills/my-skill", skills[0].Path)
	}
}

func TestPlacement_DotClaude(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, ".claude", "skills", "init-claude"), "In .claude/skills")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "init-claude" {
		t.Errorf("name = %q", skills[0].Name)
	}
}

func TestPlacement_DotGitHub(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, ".github", "skills", "init-copilot"), "In .github/skills")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "init-copilot" {
		t.Errorf("name = %q", skills[0].Name)
	}
}

func TestPlacement_DotCursor(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, ".cursor", "skills", "init-cursor"), "In .cursor/skills")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "init-cursor" {
		t.Errorf("name = %q", skills[0].Name)
	}
}

func TestPlacement_Mixed_TopLevelAndNested(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "skill-top"), "Top level")
	createSkill(t, filepath.Join(root, "core", "skills", "skill-nested"), "Nested")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	names := skillNames(skills)
	if len(names) != 2 {
		t.Fatalf("got %d skills, want 2", len(names))
	}
	if names[0] != "skill-nested" || names[1] != "skill-top" {
		t.Errorf("names = %v", names)
	}
}

func TestPlacement_Mixed_MultipleProviderDirs(t *testing.T) {
	// Simulates the dev-copilot structure: skills spread across
	// .claude/skills, .github/skills, and core/skills
	root := t.TempDir()
	createSkill(t, filepath.Join(root, ".claude", "skills", "init-claude"), "Claude init")
	createSkill(t, filepath.Join(root, ".github", "skills", "init-copilot"), "Copilot init")
	createSkill(t, filepath.Join(root, "core", "skills", "context-files"), "Context files")
	createSkill(t, filepath.Join(root, "core", "skills", "creating-pr"), "Creating PR")
	createSkill(t, filepath.Join(root, "core", "skills", "git-commit"), "Git commit")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 5 {
		t.Fatalf("got %d skills, want 5", len(skills))
	}

	// Verify each skill has the correct path
	cf := skillByName(skills, "context-files")
	if cf == nil {
		t.Fatal("context-files not found")
	}
	if cf.Path != filepath.Join(root, "core", "skills", "context-files") {
		t.Errorf("context-files path = %q", cf.Path)
	}

	ic := skillByName(skills, "init-claude")
	if ic == nil {
		t.Fatal("init-claude not found")
	}
	if ic.Path != filepath.Join(root, ".claude", "skills", "init-claude") {
		t.Errorf("init-claude path = %q", ic.Path)
	}
}

func TestPlacement_DeeplyNested(t *testing.T) {
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "a", "b", "c", "d", "deep-skill"), "Very deep")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "deep-skill" {
		t.Errorf("name = %q", skills[0].Name)
	}
}

func TestPlacement_SkillWithSubdirectories(t *testing.T) {
	// Skill directory itself contains subdirectories (references, templates, etc.)
	root := t.TempDir()
	skillDir := filepath.Join(root, "my-skill")
	createSkill(t, skillDir, "Skill with subdirs")
	os.MkdirAll(filepath.Join(skillDir, "references"), 0755)
	os.MkdirAll(filepath.Join(skillDir, "templates"), 0755)
	os.WriteFile(filepath.Join(skillDir, "references", "api.md"), []byte("ref"), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	// Should find exactly 1 skill, not be confused by subdirs
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "my-skill" {
		t.Errorf("name = %q", skills[0].Name)
	}
}

func TestPlacement_NestedSKILLMDNotInSubdirs(t *testing.T) {
	// SKILL.md inside a subdirectory of a skill should create separate skills
	root := t.TempDir()
	createSkill(t, filepath.Join(root, "parent-skill"), "Parent")
	createSkill(t, filepath.Join(root, "parent-skill", "child-skill"), "Child")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	// Should find both parent and child as separate skills
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}
	names := skillNames(skills)
	if names[0] != "child-skill" || names[1] != "parent-skill" {
		t.Errorf("names = %v", names)
	}
}

func TestPlacement_SameNameDifferentLocations(t *testing.T) {
	// Two skills with the same directory name in different subtrees
	root := t.TempDir()
	createSkill(t, filepath.Join(root, ".claude", "skills", "init"), "Claude init")
	createSkill(t, filepath.Join(root, ".github", "skills", "init"), "GitHub init")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 2 {
		t.Fatalf("got %d skills, want 2", len(skills))
	}
	// Both should be named "init" but have different paths
	for _, s := range skills {
		if s.Name != "init" {
			t.Errorf("name = %q, want init", s.Name)
		}
	}
	if skills[0].Path == skills[1].Path {
		t.Error("expected different paths for same-named skills")
	}
}

func TestPlacement_EmptySource(t *testing.T) {
	root := t.TempDir()
	// No SKILL.md files at all
	os.MkdirAll(filepath.Join(root, "src", "main"), 0755)
	os.WriteFile(filepath.Join(root, "README.md"), []byte("# Repo"), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 0 {
		t.Errorf("got %d skills, want 0", len(skills))
	}
}

func TestPlacement_SkillMDAtRoot(t *testing.T) {
	// SKILL.md directly in the repo root — the skill name would be the repo dir name
	root := t.TempDir()
	content := "---\ndescription: Root skill\n---\n"
	os.WriteFile(filepath.Join(root, "SKILL.md"), []byte(content), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	// Name should be the temp dir's basename
	if skills[0].Path != root {
		t.Errorf("path = %q, want %q", skills[0].Path, root)
	}
}

func TestPlacement_SymlinkedSkillDirectory(t *testing.T) {
	root := t.TempDir()
	// Create actual skill in one location
	actualDir := filepath.Join(root, "actual-skills", "real-skill")
	createSkill(t, actualDir, "Real skill")

	// Create symlink to it in another location
	linkParent := filepath.Join(root, "linked-skills")
	os.MkdirAll(linkParent, 0755)
	os.Symlink(actualDir, filepath.Join(linkParent, "linked-skill"))

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	// filepath.Walk does not follow symlinks to directories,
	// so only the real skill should be found
	if len(skills) != 1 {
		t.Fatalf("got %d skills, want 1", len(skills))
	}
	if skills[0].Name != "real-skill" {
		t.Errorf("name = %q, want real-skill", skills[0].Name)
	}
}

func TestPlacement_ExcludedDirsMixed(t *testing.T) {
	root := t.TempDir()

	// Skills in excluded directories (should NOT be found)
	createSkill(t, filepath.Join(root, ".git", "hooks", "my-hook"), "Git hook")
	createSkill(t, filepath.Join(root, "node_modules", "pkg", "skill"), "Node module")
	createSkill(t, filepath.Join(root, "vendor", "dep", "skill"), "Vendor dep")
	createSkill(t, filepath.Join(root, "__pycache__", "skill"), "Pycache")
	createSkill(t, filepath.Join(root, ".venv", "lib", "skill"), "Venv")

	// Skills in non-excluded dot directories (SHOULD be found)
	createSkill(t, filepath.Join(root, ".claude", "skills", "claude-skill"), "Claude")
	createSkill(t, filepath.Join(root, ".github", "skills", "gh-skill"), "GitHub")
	createSkill(t, filepath.Join(root, ".cursor", "skills", "cursor-skill"), "Cursor")

	// Regular skill
	createSkill(t, filepath.Join(root, "my-skill"), "Regular")

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}

	names := skillNames(skills)
	expected := []string{"claude-skill", "cursor-skill", "gh-skill", "my-skill"}
	if len(names) != len(expected) {
		t.Fatalf("got skills %v, want %v", names, expected)
	}
	for i := range expected {
		if names[i] != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, names[i], expected[i])
		}
	}
}

func TestPlacement_DescriptionFromFrontmatter(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "desc-skill")
	os.MkdirAll(skillDir, 0755)
	content := `---
name: desc-skill
description: "Handles context file management"
type: reference
---

# Context Files

Some content here.
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatal("expected 1 skill")
	}
	if skills[0].Description != "Handles context file management" {
		t.Errorf("description = %q", skills[0].Description)
	}
}

func TestPlacement_DescriptionFoldedYAML(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "folded-skill")
	os.MkdirAll(skillDir, 0755)
	content := `---
name: context-files
description: >
  Create or validate project context files.
  Use when bootstrapping a new project.
---

# Context Files
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatal("expected 1 skill")
	}
	expected := "Create or validate project context files. Use when bootstrapping a new project."
	if skills[0].Description != expected {
		t.Errorf("description = %q, want %q", skills[0].Description, expected)
	}
}

func TestPlacement_DescriptionLiteralYAML(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "literal-skill")
	os.MkdirAll(skillDir, 0755)
	content := `---
name: my-skill
description: |
  First line of description.
  Second line of description.
---

# My Skill
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatal("expected 1 skill")
	}
	expected := "First line of description. Second line of description."
	if skills[0].Description != expected {
		t.Errorf("description = %q, want %q", skills[0].Description, expected)
	}
}

func TestPlacement_DescriptionFallback(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "fb-skill")
	os.MkdirAll(skillDir, 0755)
	content := `# My Skill

This is the fallback description line.
`
	os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644)

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 {
		t.Fatal("expected 1 skill")
	}
	if skills[0].Description != "This is the fallback description line." {
		t.Errorf("description = %q", skills[0].Description)
	}
}

func TestPlacement_LargeSource(t *testing.T) {
	root := t.TempDir()
	// Create 50 skills across multiple directories
	for i := 0; i < 20; i++ {
		createSkill(t, filepath.Join(root, "core", "skills", "skill-"+string(rune('a'+i))), "Core skill")
	}
	for i := 0; i < 10; i++ {
		createSkill(t, filepath.Join(root, ".claude", "skills", "claude-"+string(rune('a'+i))), "Claude skill")
	}
	for i := 0; i < 5; i++ {
		createSkill(t, filepath.Join(root, "top-"+string(rune('a'+i))), "Top skill")
	}

	skills, err := ScanRepo(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 35 {
		t.Errorf("got %d skills, want 35", len(skills))
	}
}
