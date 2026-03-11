package models_test

import (
	"path/filepath"
	"testing"

	"github.com/vladyslav/skillreg/internal/db"
	"github.com/vladyslav/skillreg/internal/models"
)

// newTestDB opens an in-memory (temp dir) SQLite database for testing.
func newTestDB(t *testing.T) *db.Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(path)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

func TestCreateSource(t *testing.T) {
	d := newTestDB(t)

	s, err := models.CreateSource(d, "my-skills", "/home/user/skills", "https://github.com/user/skills")
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	if s.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if s.Name != "my-skills" {
		t.Errorf("Name = %q, want %q", s.Name, "my-skills")
	}
	if s.Path != "/home/user/skills" {
		t.Errorf("Path = %q, want %q", s.Path, "/home/user/skills")
	}
	if s.RemoteURL != "https://github.com/user/skills" {
		t.Errorf("RemoteURL = %q, want %q", s.RemoteURL, "https://github.com/user/skills")
	}
	if s.AutoUpdate {
		t.Error("expected AutoUpdate = false by default")
	}
	if s.LastCheckedAt != nil {
		t.Error("expected LastCheckedAt = nil by default")
	}
	if s.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestGetSource(t *testing.T) {
	d := newTestDB(t)

	created, _ := models.CreateSource(d, "test", "/tmp/test", "")
	got, err := models.GetSource(d, created.ID)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}
}

func TestListSources(t *testing.T) {
	d := newTestDB(t)

	models.CreateSource(d, "a", "/tmp/a", "")
	models.CreateSource(d, "b", "/tmp/b", "")

	list, err := models.ListSources(d)
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestDeleteSource(t *testing.T) {
	d := newTestDB(t)

	s, _ := models.CreateSource(d, "del", "/tmp/del", "")
	if err := models.DeleteSource(d, s.ID); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}
	list, _ := models.ListSources(d)
	if len(list) != 0 {
		t.Errorf("expected 0 sources after delete, got %d", len(list))
	}
}

func TestSetSourceAutoUpdate(t *testing.T) {
	d := newTestDB(t)

	s, _ := models.CreateSource(d, "au", "/tmp/au", "")
	if err := models.SetSourceAutoUpdate(d, s.ID, true); err != nil {
		t.Fatalf("SetSourceAutoUpdate true: %v", err)
	}
	got, _ := models.GetSource(d, s.ID)
	if !got.AutoUpdate {
		t.Error("expected AutoUpdate = true")
	}

	models.SetSourceAutoUpdate(d, s.ID, false)
	got, _ = models.GetSource(d, s.ID)
	if got.AutoUpdate {
		t.Error("expected AutoUpdate = false after toggle")
	}
}

func TestUpdateSourceLastChecked(t *testing.T) {
	d := newTestDB(t)

	s, _ := models.CreateSource(d, "lc", "/tmp/lc", "")
	if err := models.UpdateSourceLastChecked(d, s.ID); err != nil {
		t.Fatalf("UpdateSourceLastChecked: %v", err)
	}
	got, _ := models.GetSource(d, s.ID)
	if got.LastCheckedAt == nil {
		t.Error("expected LastCheckedAt to be set")
	}
}

func TestSourceUniquePathConstraint(t *testing.T) {
	d := newTestDB(t)

	models.CreateSource(d, "s1", "/tmp/same", "")
	_, err := models.CreateSource(d, "s2", "/tmp/same", "")
	if err == nil {
		t.Error("expected error for duplicate path, got nil")
	}
}
