package models_test

import (
	"testing"

	"github.com/vladyslav/skillreg/internal/models"
)

// createTestProvider is a helper that creates a provider for use in instance tests.
func createTestProvider(t *testing.T, d interface {
	/* db.Database accessible via models */ }) int64 {
	t.Helper()
	return 0 // unused; see actual helper below
}

func mustCreateProvider(t *testing.T, d interface{ DB() interface{} }, name string) int64 {
	t.Helper()
	return 0
}

func TestCreateInstance(t *testing.T) {
	d := newTestDB(t)

	// Create a provider first (foreign key requirement)
	prov, err := models.CreateProvider(d, "InstanceProv", ".iprov")
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}

	inst, err := models.CreateInstance(d, prov.ID, "main", "/home/user/.iprov/skills", true)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if inst.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if inst.ProviderID != prov.ID {
		t.Errorf("ProviderID = %d, want %d", inst.ProviderID, prov.ID)
	}
	if inst.Name != "main" {
		t.Errorf("Name = %q, want %q", inst.Name, "main")
	}
	if inst.GlobalSkillsPath != "/home/user/.iprov/skills" {
		t.Errorf("GlobalSkillsPath = %q", inst.GlobalSkillsPath)
	}
	if !inst.IsDefault {
		t.Error("expected IsDefault = true")
	}
	if inst.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestGetInstance(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "GP", ".gp")
	created, _ := models.CreateInstance(d, prov.ID, "gp-main", "/gp/skills", false)
	got, err := models.GetInstance(d, created.ID)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}
}

func TestListInstancesByProvider(t *testing.T) {
	d := newTestDB(t)

	p1, _ := models.CreateProvider(d, "Prov1", ".p1")
	p2, _ := models.CreateProvider(d, "Prov2", ".p2")

	models.CreateInstance(d, p1.ID, "p1-a", "/p1/a", false)
	models.CreateInstance(d, p1.ID, "p1-b", "/p1/b", false)
	models.CreateInstance(d, p2.ID, "p2-a", "/p2/a", false)

	list, err := models.ListInstancesByProvider(d, p1.ID)
	if err != nil {
		t.Fatalf("ListInstancesByProvider: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
	for _, inst := range list {
		if inst.ProviderID != p1.ID {
			t.Errorf("unexpected ProviderID %d", inst.ProviderID)
		}
	}
}

func TestListAllInstances(t *testing.T) {
	d := newTestDB(t)

	p, _ := models.CreateProvider(d, "AllProv", ".allp")
	models.CreateInstance(d, p.ID, "all-a", "/all/a", false)
	models.CreateInstance(d, p.ID, "all-b", "/all/b", false)

	list, err := models.ListAllInstances(d)
	if err != nil {
		t.Fatalf("ListAllInstances: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("len = %d, want >= 2", len(list))
	}
}

func TestDeleteInstance(t *testing.T) {
	d := newTestDB(t)

	p, _ := models.CreateProvider(d, "DelProv", ".delp")
	inst, _ := models.CreateInstance(d, p.ID, "del-inst", "/del/inst", false)

	if err := models.DeleteInstance(d, inst.ID); err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	list, _ := models.ListInstancesByProvider(d, p.ID)
	if len(list) != 0 {
		t.Errorf("expected 0 instances after delete, got %d", len(list))
	}
}

func TestInstanceDuplicateGlobalSkillsPath(t *testing.T) {
	d := newTestDB(t)

	p, _ := models.CreateProvider(d, "DupPath", ".dupp")
	models.CreateInstance(d, p.ID, "inst-1", "/dup/path", false)
	_, err := models.CreateInstance(d, p.ID, "inst-2", "/dup/path", false)
	if err == nil {
		t.Error("expected error for duplicate global_skills_path, got nil")
	}
}
