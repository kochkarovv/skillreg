package models_test

import (
	"testing"

	"github.com/vladyslav/skillreg/internal/models"
)

// setupInstallationFixtures creates a provider, instance, source, and skill
// so that installation tests have all required foreign keys in place.
func setupInstallationFixtures(t *testing.T) (instanceID, skillID int64) {
	t.Helper()
	d := newTestDB(t)

	prov, err := models.CreateProvider(d, "InstProv", ".instp")
	if err != nil {
		t.Fatalf("CreateProvider: %v", err)
	}
	inst, err := models.CreateInstance(d, prov.ID, "inst-fix", "/inst/fix", false)
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	src, err := models.CreateSource(d, "inst-src", "/inst/src", "")
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	sk, err := models.CreateSkill(d, src.ID, "inst-skill", "/inst/src/skill.md", "")
	if err != nil {
		t.Fatalf("CreateSkill: %v", err)
	}
	return inst.ID, sk.ID
}

func TestCreateInstallation(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "CIProv", ".ci")
	inst, _ := models.CreateInstance(d, prov.ID, "ci-inst", "/ci/inst", false)
	src, _ := models.CreateSource(d, "ci-src", "/ci/src", "")
	sk, _ := models.CreateSkill(d, src.ID, "ci-skill", "/ci/src/skill.md", "")

	instn, err := models.CreateInstallation(d, sk.ID, inst.ID, "/ci/inst/skill.md", "ci-skill")
	if err != nil {
		t.Fatalf("CreateInstallation: %v", err)
	}
	if instn.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if instn.SkillID != sk.ID {
		t.Errorf("SkillID = %d, want %d", instn.SkillID, sk.ID)
	}
	if instn.InstanceID != inst.ID {
		t.Errorf("InstanceID = %d, want %d", instn.InstanceID, inst.ID)
	}
	if instn.SymlinkPath != "/ci/inst/skill.md" {
		t.Errorf("SymlinkPath = %q", instn.SymlinkPath)
	}
	if instn.InstalledName != "ci-skill" {
		t.Errorf("InstalledName = %q", instn.InstalledName)
	}
	if instn.InstalledAt.IsZero() {
		t.Error("expected non-zero InstalledAt")
	}
	if instn.Status != "active" {
		t.Errorf("Status = %q, want %q", instn.Status, "active")
	}
}

func TestGetInstallation(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "GIProv", ".gi")
	inst, _ := models.CreateInstance(d, prov.ID, "gi-inst", "/gi/inst", false)
	src, _ := models.CreateSource(d, "gi-src", "/gi/src", "")
	sk, _ := models.CreateSkill(d, src.ID, "gi-skill", "/gi/src/skill.md", "")
	created, _ := models.CreateInstallation(d, sk.ID, inst.ID, "/gi/inst/skill.md", "gi-skill")

	got, err := models.GetInstallation(d, created.ID)
	if err != nil {
		t.Fatalf("GetInstallation: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %d, want %d", got.ID, created.ID)
	}
}

func TestListInstallationsByInstance(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "LIProv", ".li")
	inst1, _ := models.CreateInstance(d, prov.ID, "li-inst1", "/li/inst1", false)
	inst2, _ := models.CreateInstance(d, prov.ID, "li-inst2", "/li/inst2", false)
	src, _ := models.CreateSource(d, "li-src", "/li/src", "")
	sk1, _ := models.CreateSkill(d, src.ID, "li-sk1", "/li/src/sk1.md", "")
	sk2, _ := models.CreateSkill(d, src.ID, "li-sk2", "/li/src/sk2.md", "")

	models.CreateInstallation(d, sk1.ID, inst1.ID, "/li/inst1/sk1.md", "li-sk1")
	models.CreateInstallation(d, sk2.ID, inst1.ID, "/li/inst1/sk2.md", "li-sk2")
	models.CreateInstallation(d, sk1.ID, inst2.ID, "/li/inst2/sk1.md", "li-sk1")

	list, err := models.ListInstallationsByInstance(d, inst1.ID)
	if err != nil {
		t.Fatalf("ListInstallationsByInstance: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestListInstallationsBySkill(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "LISProv", ".lis")
	inst1, _ := models.CreateInstance(d, prov.ID, "lis-inst1", "/lis/inst1", false)
	inst2, _ := models.CreateInstance(d, prov.ID, "lis-inst2", "/lis/inst2", false)
	src, _ := models.CreateSource(d, "lis-src", "/lis/src", "")
	sk, _ := models.CreateSkill(d, src.ID, "lis-skill", "/lis/src/skill.md", "")

	models.CreateInstallation(d, sk.ID, inst1.ID, "/lis/inst1/skill.md", "lis-skill")
	models.CreateInstallation(d, sk.ID, inst2.ID, "/lis/inst2/skill.md", "lis-skill")

	list, err := models.ListInstallationsBySkill(d, sk.ID)
	if err != nil {
		t.Fatalf("ListInstallationsBySkill: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}
}

func TestListAllInstallations(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "LAProv", ".la")
	inst, _ := models.CreateInstance(d, prov.ID, "la-inst", "/la/inst", false)
	src, _ := models.CreateSource(d, "la-src", "/la/src", "")
	sk1, _ := models.CreateSkill(d, src.ID, "la-sk1", "/la/src/sk1.md", "")
	sk2, _ := models.CreateSkill(d, src.ID, "la-sk2", "/la/src/sk2.md", "")

	models.CreateInstallation(d, sk1.ID, inst.ID, "/la/inst/sk1.md", "la-sk1")
	models.CreateInstallation(d, sk2.ID, inst.ID, "/la/inst/sk2.md", "la-sk2")

	list, err := models.ListAllInstallations(d)
	if err != nil {
		t.Fatalf("ListAllInstallations: %v", err)
	}
	if len(list) < 2 {
		t.Errorf("len = %d, want >= 2", len(list))
	}
}

func TestUpdateInstallationStatus(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "UISProv", ".uis")
	inst, _ := models.CreateInstance(d, prov.ID, "uis-inst", "/uis/inst", false)
	src, _ := models.CreateSource(d, "uis-src", "/uis/src", "")
	sk, _ := models.CreateSkill(d, src.ID, "uis-skill", "/uis/src/skill.md", "")
	instn, _ := models.CreateInstallation(d, sk.ID, inst.ID, "/uis/inst/skill.md", "uis-skill")

	if err := models.UpdateInstallationStatus(d, instn.ID, "broken"); err != nil {
		t.Fatalf("UpdateInstallationStatus: %v", err)
	}
	got, _ := models.GetInstallation(d, instn.ID)
	if got.Status != "broken" {
		t.Errorf("Status = %q, want %q", got.Status, "broken")
	}
}

func TestDeleteInstallation(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "DIProv", ".di")
	inst, _ := models.CreateInstance(d, prov.ID, "di-inst", "/di/inst", false)
	src, _ := models.CreateSource(d, "di-src", "/di/src", "")
	sk, _ := models.CreateSkill(d, src.ID, "di-skill", "/di/src/skill.md", "")
	instn, _ := models.CreateInstallation(d, sk.ID, inst.ID, "/di/inst/skill.md", "di-skill")

	if err := models.DeleteInstallation(d, instn.ID); err != nil {
		t.Fatalf("DeleteInstallation: %v", err)
	}
	list, _ := models.ListInstallationsByInstance(d, inst.ID)
	if len(list) != 0 {
		t.Errorf("expected 0 installations after delete, got %d", len(list))
	}
}

func TestInstallationDuplicateSkillInstance(t *testing.T) {
	d := newTestDB(t)

	prov, _ := models.CreateProvider(d, "DupInstProv", ".dupi")
	inst, _ := models.CreateInstance(d, prov.ID, "dupi-inst", "/dupi/inst", false)
	src, _ := models.CreateSource(d, "dupi-src", "/dupi/src", "")
	sk, _ := models.CreateSkill(d, src.ID, "dupi-skill", "/dupi/src/skill.md", "")

	models.CreateInstallation(d, sk.ID, inst.ID, "/dupi/inst/skill.md", "dupi-skill")
	_, err := models.CreateInstallation(d, sk.ID, inst.ID, "/dupi/inst/skill2.md", "dupi-skill2")
	if err == nil {
		t.Error("expected error for duplicate skill_id+instance_id, got nil")
	}
}
