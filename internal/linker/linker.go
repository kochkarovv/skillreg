package linker

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Status constants for symlink checks
const (
	StatusActive   = "active"
	StatusBroken   = "broken"
	StatusOrphaned = "orphaned"
)

// CreateSymlink creates a symlink from target pointing to source.
// Creates parent directories if they don't exist.
func CreateSymlink(source, target string) error {
	// Create parent directories if needed
	parentDir := filepath.Dir(target)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directories for symlink: %w", err)
	}

	// Create the symlink
	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("failed to create symlink from %q to %q: %w", target, source, err)
	}

	return nil
}

// RemoveSymlink removes a symlink at the given path.
func RemoveSymlink(target string) error {
	if err := os.Remove(target); err != nil {
		return fmt.Errorf("failed to remove symlink at %q: %w", target, err)
	}
	return nil
}

// BackupAndReplace backs up an existing directory as a zip file,
// removes the original directory, and creates a symlink in its place.
func BackupAndReplace(source, target string) error {
	// Only proceed if target exists
	if !ExistsAtTarget(target) {
		return fmt.Errorf("target %q does not exist", target)
	}

	// Create backup zip file
	backupPath := target + ".skill.bak.zip"
	if err := zipDirectory(target, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Remove original directory
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("failed to remove original directory: %w", err)
	}

	// Create symlink
	if err := CreateSymlink(source, target); err != nil {
		return fmt.Errorf("failed to create symlink after backup: %w", err)
	}

	return nil
}

// CheckSymlink checks the status of a symlink.
// Returns:
// - StatusActive if symlink exists and points to expectedTarget
// - StatusBroken if symlink exists but doesn't point to expectedTarget or target is gone
// - StatusOrphaned if symlink doesn't exist on disk
func CheckSymlink(symlinkPath, expectedTarget string) string {
	// Check if symlink exists on disk using Lstat (doesn't follow symlinks)
	fileInfo, err := os.Lstat(symlinkPath)
	if err != nil {
		// If any error (not found, permission denied, etc.), it's orphaned
		return StatusOrphaned
	}

	// Check if it's actually a symlink
	if fileInfo.Mode()&os.ModeSymlink == 0 {
		// Exists but is not a symlink
		return StatusOrphaned
	}

	// Read where the symlink actually points
	actualTarget, err := os.Readlink(symlinkPath)
	if err != nil {
		// Can't read the symlink, it's broken
		return StatusBroken
	}

	// Check if symlink points to the expected target
	if actualTarget != expectedTarget {
		// Points to wrong target
		return StatusBroken
	}

	// Check if the target actually exists
	if !ExistsAtTarget(expectedTarget) {
		// Target is gone, symlink is broken/dangling
		return StatusBroken
	}

	return StatusActive
}

// IsSymlink returns true if the path is a symlink.
func IsSymlink(path string) bool {
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return fileInfo.Mode()&os.ModeSymlink != 0
}

// IsDirectory returns true if the path is a directory.
func IsDirectory(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

// ExistsAtTarget returns true if the path exists (file, directory, or symlink).
func ExistsAtTarget(target string) bool {
	_, err := os.Stat(target)
	return err == nil
}

// zipDirectory creates a zip file from a directory.
func zipDirectory(source, target string) error {
	// Create the zip file
	zipFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create zip file: %w", err)
	}
	defer zipFile.Close()

	// Create zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through source directory
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path for zip entry
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Set the header name (use forward slashes for zip compatibility)
		header.Name = filepath.ToSlash(relPath)

		// Handle directories vs files
		if info.IsDir() {
			header.Name += "/"
		}

		// Write header to zip
		w, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		// For files, write content
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(w, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	return nil
}
