package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoveredSkill represents a skill found during repository scanning
type DiscoveredSkill struct {
	Name        string // immediate parent directory name of SKILL.md
	Path        string // absolute path to the skill directory
	Description string // parsed description from SKILL.md
}

// excludedDirs map of directories that should not be scanned
var excludedDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"vendor":      true,
	"__pycache__": true,
	".venv":       true,
}

// ScanRepo walks the directory tree, finds all SKILL.md files, and returns discovered skills
func ScanRepo(repoPath string) ([]DiscoveredSkill, error) {
	// Verify the repo path exists
	info, err := os.Stat(repoPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("repoPath is not a directory: %s", repoPath)
	}

	var skills []DiscoveredSkill

	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check if this directory should be excluded
		if info.IsDir() && isExcludedDir(info.Name()) && path != repoPath {
			return filepath.SkipDir
		}

		// Check if this is a SKILL.md file
		if info.Name() == "SKILL.md" && !info.IsDir() {
			// Get the parent directory name (the skill name)
			skillDir := filepath.Dir(path)
			skillName := filepath.Base(skillDir)

			// Read the SKILL.md file
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Parse the description
			description := ParseDescription(string(content))

			// Add to results
			skills = append(skills, DiscoveredSkill{
				Name:        skillName,
				Path:        skillDir,
				Description: description,
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return skills, nil
}

// ParseDescription extracts description from SKILL.md content
// First tries YAML frontmatter `description:` field
// Falls back to first non-empty, non-heading line (not starting with `#`)
// Returns empty string if content is empty or malformed
func ParseDescription(content string) string {
	lines := strings.Split(content, "\n")

	// Try to parse YAML frontmatter first
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		// Look for description: field in frontmatter
		for i := 1; i < len(lines); i++ {
			line := lines[i]

			// End of frontmatter
			if strings.TrimSpace(line) == "---" {
				break
			}

			// Check for description field
			if strings.HasPrefix(strings.TrimSpace(line), "description:") {
				// Extract the value after "description:"
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					desc := strings.TrimSpace(parts[1])
					// Remove quotes if present
					desc = strings.Trim(desc, "\"'")

					// Handle YAML multiline scalars (> folded, | literal)
					if desc == ">" || desc == "|" || desc == ">-" || desc == "|-" {
						var multiParts []string
						for j := i + 1; j < len(lines); j++ {
							ml := lines[j]
							if strings.TrimSpace(ml) == "---" {
								break
							}
							// Continuation lines are indented
							if len(ml) > 0 && (ml[0] == ' ' || ml[0] == '\t') {
								multiParts = append(multiParts, strings.TrimSpace(ml))
							} else {
								break
							}
						}
						if len(multiParts) > 0 {
							return strings.Join(multiParts, " ")
						}
						// Empty multiline block, skip
						continue
					}

					if desc != "" {
						return desc
					}
				}
			}
		}
	}

	// Fall back to first non-empty, non-heading line
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines and headings (lines starting with #)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return trimmed
		}
	}

	return ""
}

// isExcludedDir checks if a directory name should be excluded from scanning
func isExcludedDir(dirName string) bool {
	return excludedDirs[dirName]
}
