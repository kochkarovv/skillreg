package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestRepo creates a real git repo in a temp directory for testing
func setupTestRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	// Configure user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config user email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config user name: %v", err)
	}

	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cmd = exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create initial commit: %v", err)
	}

	return tmpDir
}

func TestIsGitRepo(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() string
		expected bool
	}{
		{
			name: "valid git repo",
			setup: func() string {
				return setupTestRepo(t)
			},
			expected: true,
		},
		{
			name: "non-git directory",
			setup: func() string {
				tmpDir := t.TempDir()
				return tmpDir
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			result := IsGitRepo(path)
			if result != tt.expected {
				t.Errorf("IsGitRepo(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestGetRemoteURL(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() string
		expected string
	}{
		{
			name: "repo with no remote",
			setup: func() string {
				return setupTestRepo(t)
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			result := GetRemoteURL(path)
			if result != tt.expected {
				t.Errorf("GetRemoteURL(%q) = %q, want %q", path, result, tt.expected)
			}
		})
	}
}

func TestGetCurrentBranch(t *testing.T) {
	repoPath := setupTestRepo(t)

	branch, err := GetCurrentBranch(repoPath)
	if err != nil {
		t.Fatalf("GetCurrentBranch() failed: %v", err)
	}

	// Default branch after git init can be 'master' or 'main' depending on git config
	if branch != "master" && branch != "main" {
		t.Errorf("GetCurrentBranch() = %q, want 'master' or 'main'", branch)
	}
}

func TestIsDirty(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() string
		expected bool
	}{
		{
			name: "clean repository",
			setup: func() string {
				return setupTestRepo(t)
			},
			expected: false,
		},
		{
			name: "dirty repository with uncommitted changes",
			setup: func() string {
				repoPath := setupTestRepo(t)
				// Create an uncommitted change
				testFile := filepath.Join(repoPath, "test.txt")
				if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
					t.Fatalf("Failed to modify file: %v", err)
				}
				return repoPath
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			result, err := IsDirty(path)
			if err != nil {
				t.Fatalf("IsDirty() failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("IsDirty(%q) = %v, want %v", path, result, tt.expected)
			}
		})
	}
}

func TestFetch(t *testing.T) {
	// Test that Fetch doesn't error on a local repo without remote
	repoPath := setupTestRepo(t)

	err := Fetch(repoPath)
	if err != nil {
		t.Errorf("Fetch() failed on local repo without remote: %v", err)
	}
}

func TestCommitsBehind(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() string
		expected int
	}{
		{
			name: "repo with no remote",
			setup: func() string {
				return setupTestRepo(t)
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			result, err := CommitsBehind(path)
			if err != nil {
				t.Fatalf("CommitsBehind() failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("CommitsBehind(%q) = %d, want %d", path, result, tt.expected)
			}
		})
	}
}
