package models_test

import (
	"testing"

	"github.com/vladyslav/skillreg/internal/models"
)

func TestCreateProvider(t *testing.T) {
	d := newTestDB(t)

	p, err := models.CreateProvider(d, "MyTool", ".mytool")
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	if p.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if p.Name != "MyTool" {
		t.Errorf("Name = %q, want %q", p.Name, "MyTool")
	}
	if p.ConfigDirPrefix != ".mytool" {
		t.Errorf("ConfigDirPrefix = %q, want %q", p.ConfigDirPrefix, ".mytool")
	}
	if p.IsBuiltin {
		t.Error("expected IsBuiltin = false for user-created provider")
	}
}

func TestGetProvider(t *testing.T) {
	d := newTestDB(t)

	created, _ := models.CreateProvider(d, "GetTest", ".gettest")
	got, err := models.GetProvider(d, created.ID)
	if err != nil {
		t.Fatalf("GetProvider: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}
}

func TestListProviders(t *testing.T) {
	d := newTestDB(t)

	models.CreateProvider(d, "ToolA", ".toola")
	models.CreateProvider(d, "ToolB", ".toolb")

	list, err := models.ListProviders(d)
	if err != nil {
		t.Fatalf("ListProviders: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("len = %d, want >= 2", len(list))
	}
}

func TestProviderUniqueNameConstraint(t *testing.T) {
	d := newTestDB(t)

	models.CreateProvider(d, "Duplicate", ".dup")
	_, err := models.CreateProvider(d, "Duplicate", ".dup2")
	if err == nil {
		t.Error("expected error for duplicate provider name, got nil")
	}
}
