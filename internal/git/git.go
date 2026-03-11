package git

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// runGit is a helper function that runs a git command in the specified repository.
// It is unexported and used internally by other functions in this package.
func runGit(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = repoPath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// IsGitRepo checks whether the given path is a valid git repository.
func IsGitRepo(path string) bool {
	gitDir := path + "/.git"
	_, err := os.Stat(gitDir)
	return err == nil
}

// GetRemoteURL returns the URL of the "origin" remote.
// Returns empty string if no remote is configured.
func GetRemoteURL(path string) string {
	output, err := runGit(path, "config", "--get", "remote.origin.url")
	if err != nil {
		return ""
	}
	return output
}

// GetCurrentBranch returns the name of the current branch.
func GetCurrentBranch(path string) (string, error) {
	return runGit(path, "rev-parse", "--abbrev-ref", "HEAD")
}

// IsDirty checks whether the repository has uncommitted changes.
func IsDirty(path string) (bool, error) {
	output, err := runGit(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return output != "", nil
}

// Fetch fetches updates from the remote repository.
// If there is no remote, it returns without error.
func Fetch(path string) error {
	// Ignore errors for repos without remotes
	_, _ = runGit(path, "fetch")
	return nil
}

// CommitsBehind compares the current branch HEAD with origin/<branch>.
// Returns the number of commits the local branch is behind.
// Returns 0 if there is no remote configured.
func CommitsBehind(path string) (int, error) {
	// Check if origin remote exists
	if GetRemoteURL(path) == "" {
		return 0, nil
	}

	// Get current branch
	branch, err := GetCurrentBranch(path)
	if err != nil {
		return 0, err
	}

	// Count commits behind
	output, err := runGit(path, "rev-list", "--count", "HEAD..origin/"+branch)
	if err != nil {
		// If the remote branch doesn't exist, return 0
		return 0, nil
	}

	count, err := strconv.Atoi(output)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commits behind: %w", err)
	}

	return count, nil
}

// PullFF performs a git pull with --ff-only flag.
// This only allows fast-forward merges.
func PullFF(path string) error {
	_, err := runGit(path, "pull", "--ff-only")
	return err
}

// StashAndPull stashes uncommitted changes, pulls the latest changes,
// and pops the stash.
func StashAndPull(path string) error {
	// Stash changes
	_, err := runGit(path, "stash")
	if err != nil {
		return fmt.Errorf("failed to stash: %w", err)
	}

	// Pull changes
	_, err = runGit(path, "pull")
	if err != nil {
		// Try to pop stash even if pull failed
		runGit(path, "stash", "pop")
		return fmt.Errorf("failed to pull: %w", err)
	}

	// Pop stash
	_, err = runGit(path, "stash", "pop")
	if err != nil {
		return fmt.Errorf("failed to pop stash: %w", err)
	}

	return nil
}

// ForceReset performs a hard reset to origin/<current-branch>.
func ForceReset(path string) error {
	branch, err := GetCurrentBranch(path)
	if err != nil {
		return err
	}

	_, err = runGit(path, "reset", "--hard", "origin/"+branch)
	return err
}
