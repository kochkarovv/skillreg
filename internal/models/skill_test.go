package models_test

import (
	"testing"

	"github.com/vladyslav/skillreg/internal/models"
)

func TestCreateSkill(t *testing.T) {
	d := newTestDB(t)

	src, err := models.CreateSource(d, "skills-src", "/skills/src", "")
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	sk, err := models.CreateSkill(d, src.ID, "my-skill", "/skills/src/my-skill.md", "A useful skill")
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	if sk.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if sk.SourceID != src.ID {
		t.Errorf("SourceID = %d, want %d", sk.SourceID, src.ID)
	}
	if sk.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", sk.Name, "my-skill")
	}
	if sk.OriginalPath != "/skills/src/my-skill.md" {
		t.Errorf("OriginalPath = %q", sk.OriginalPath)
	}
	if sk.Description != "A useful skill" {
		t.Errorf("Description = %q", sk.Description)
	}
	if sk.DiscoveredAt.IsZero() {
		t.Error("expected non-zero DiscoveredAt")
	}
}

func TestGetSkill(t *testing.T) {
	d := newTestDB(t)

	src, _ := models.CreateSource(d, "gs-src", "/gs/src", "")
	created, _ := models.CreateSkill(d, src.ID, "gs-skill", "/gs/src/gs.md", "")
	got, err := models.GetSkill(d, created.ID)
	if err != nil {
		t.Fatalf("GetSkill: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}
}

func TestListSkillsBySource(t *testing.T) {
	d := newTestDB(t)

	s1, _ := models.CreateSource(d, "src1", "/src1", "")
	s2, _ := models.CreateSource(d, "src2", "/src2", "")

	models.CreateSkill(d, s1.ID, "skill-a", "/src1/a.md", "")
	models.CreateSkill(d, s1.ID, "skill-b", "/src1/b.md", "")
	models.CreateSkill(d, s2.ID, "skill-c", "/src2/c.md", "")

	list, err := models.ListSkillsBySource(d, s1.ID)
	if err != nil {
		t.Fatalf("ListSkillsBySource: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestListAllSkills(t *testing.T) {
	d := newTestDB(t)

	src, _ := models.CreateSource(d, "all-src", "/all/src", "")
	models.CreateSkill(d, src.ID, "all-a", "/all/src/a.md", "")
	models.CreateSkill(d, src.ID, "all-b", "/all/src/b.md", "")

	list, err := models.ListAllSkills(d)
	if err != nil {
		t.Fatalf("ListAllSkills: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("len = %d, want >= 2", len(list))
	}
}

func TestDeleteSkillsBySource(t *testing.T) {
	d := newTestDB(t)

	src, _ := models.CreateSource(d, "del-src", "/del/src", "")
	models.CreateSkill(d, src.ID, "del-a", "/del/src/a.md", "")
	models.CreateSkill(d, src.ID, "del-b", "/del/src/b.md", "")

	if err := models.DeleteSkillsBySource(d, src.ID); err != nil {
		t.Fatalf("DeleteSkillsBySource: %v", err)
	}
	list, _ := models.ListSkillsBySource(d, src.ID)
	if len(list) != 0 {
		t.Errorf("expected 0 skills after delete, got %d", len(list))
	}
}

func TestSkillDuplicateSourceIDOriginalPath(t *testing.T) {
	d := newTestDB(t)

	src, _ := models.CreateSource(d, "dup-src", "/dup/src", "")
	models.CreateSkill(d, src.ID, "dup", "/dup/src/dup.md", "")
	_, err := models.CreateSkill(d, src.ID, "dup2", "/dup/src/dup.md", "")
	if err == nil {
		t.Error("expected error for duplicate source_id+original_path, got nil")
	}
}
