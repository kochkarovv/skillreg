package linker

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateSymlinkNewTarget(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "link", "target")

	// Create source directory
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create symlink
	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	// Verify symlink exists
	if !IsSymlink(target) {
		t.Error("Symlink was not created")
	}

	// Verify symlink points to correct target
	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if link != source {
		t.Errorf("Symlink points to %q, want %q", link, source)
	}
}

func TestCreateSymlinkCreatesParentDirs(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "deep", "nested", "path", "link")

	// Create source
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create symlink with nested parent dirs
	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("CreateSymlink failed: %v", err)
	}

	// Verify parent directories were created
	parentDir := filepath.Dir(target)
	if !IsDirectory(parentDir) {
		t.Error("Parent directories were not created")
	}

	// Verify symlink exists
	if !IsSymlink(target) {
		t.Error("Symlink was not created")
	}
}

func TestRemoveSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "link")

	// Create source and symlink
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Remove symlink
	if err := RemoveSymlink(target); err != nil {
		t.Fatalf("RemoveSymlink failed: %v", err)
	}

	// Verify symlink is gone
	if IsSymlink(target) {
		t.Error("Symlink still exists after removal")
	}
}

func TestBackupAndReplaceExistingDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "target")

	// Create source
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create target directory with some content
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(target, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Perform backup and replace
	if err := BackupAndReplace(source, target); err != nil {
		t.Fatalf("BackupAndReplace failed: %v", err)
	}

	// Verify backup zip was created
	backupPath := target + ".skill.bak.zip"
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("Backup zip not created at %q: %v", backupPath, err)
	}

	// Verify symlink now points to source
	if !IsSymlink(target) {
		t.Error("Symlink was not created")
	}

	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if link != source {
		t.Errorf("Symlink points to %q, want %q", link, source)
	}

	// Verify the symlink status is active
	status := CheckSymlink(target, source)
	if status != StatusActive {
		t.Errorf("CheckSymlink returned %q, want %q", status, StatusActive)
	}
}

func TestCheckSymlinkActive(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "link")

	// Create source and symlink
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	if err := CreateSymlink(source, target); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Check symlink status
	status := CheckSymlink(target, source)
	if status != StatusActive {
		t.Errorf("CheckSymlink returned %q, want %q", status, StatusActive)
	}
}

func TestCheckSymlinkBrokenDangling(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "link")

	// Create symlink pointing to non-existent source
	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("Failed to create dangling symlink: %v", err)
	}

	// Check symlink status
	status := CheckSymlink(target, source)
	if status != StatusBroken {
		t.Errorf("CheckSymlink returned %q, want %q for dangling symlink", status, StatusBroken)
	}
}

func TestCheckSymlinkBrokenWrongTarget(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	wrongSource := filepath.Join(tmpDir, "wrong_source")
	target := filepath.Join(tmpDir, "link")

	// Create both sources
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}
	if err := os.MkdirAll(wrongSource, 0755); err != nil {
		t.Fatalf("Failed to create wrong source: %v", err)
	}

	// Create symlink pointing to wrong source
	if err := os.Symlink(wrongSource, target); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Check symlink status - should be broken because it points to wrong target
	status := CheckSymlink(target, source)
	if status != StatusBroken {
		t.Errorf("CheckSymlink returned %q, want %q for wrong target", status, StatusBroken)
	}
}

func TestCheckSymlinkOrphaned(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	target := filepath.Join(tmpDir, "link")

	// Don't create symlink at all
	// Check symlink status
	status := CheckSymlink(target, source)
	if status != StatusOrphaned {
		t.Errorf("CheckSymlink returned %q, want %q for non-existent symlink", status, StatusOrphaned)
	}
}

func TestIsSymlink(t *testing.T) {
	tmpDir := t.TempDir()
	source := filepath.Join(tmpDir, "source")
	link := filepath.Join(tmpDir, "link")
	regularFile := filepath.Join(tmpDir, "file.txt")

	// Create source
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatalf("Failed to create source: %v", err)
	}

	// Create symlink
	if err := os.Symlink(source, link); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create regular file
	if err := ioutil.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Test IsSymlink
	if !IsSymlink(link) {
		t.Error("IsSymlink returned false for symlink")
	}

	if IsSymlink(source) {
		t.Error("IsSymlink returned true for directory")
	}

	if IsSymlink(regularFile) {
		t.Error("IsSymlink returned true for regular file")
	}

	if IsSymlink(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("IsSymlink returned true for non-existent path")
	}
}

func TestIsDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dir := filepath.Join(tmpDir, "dir")
	file := filepath.Join(tmpDir, "file.txt")

	// Create directory
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create file
	if err := ioutil.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Test IsDirectory
	if !IsDirectory(dir) {
		t.Error("IsDirectory returned false for directory")
	}

	if IsDirectory(file) {
		t.Error("IsDirectory returned true for file")
	}

	if IsDirectory(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("IsDirectory returned true for non-existent path")
	}
}

func TestExistsAtTarget(t *testing.T) {
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	existingFile := filepath.Join(tmpDir, "file.txt")

	// Create directory
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Create file
	if err := ioutil.WriteFile(existingFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Test ExistsAtTarget
	if !ExistsAtTarget(existingDir) {
		t.Error("ExistsAtTarget returned false for existing directory")
	}

	if !ExistsAtTarget(existingFile) {
		t.Error("ExistsAtTarget returned false for existing file")
	}

	if ExistsAtTarget(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("ExistsAtTarget returned true for non-existent path")
	}
}
