// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: MPL-2.0

package loader

import (
	"bufio"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const servicePathPrefix = "internal/services/"

var (
	hunkRegex = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

	// globalChangeSet holds the current loaded ChangeSet
	// Set once by LoadChanges() before analyzers run, then only read by analyzers
	globalChangeSet *ChangeSet
)

// ChangeSet represents a set of changes loaded from a source
type ChangeSet struct {
	changedLines map[string]map[int]bool
	changedFiles map[string]bool
	newFiles     map[string]bool
}

// NewChangeSet creates a new empty ChangeSet
func NewChangeSet() *ChangeSet {
	return &ChangeSet{
		changedLines: make(map[string]map[int]bool),
		changedFiles: make(map[string]bool),
		newFiles:     make(map[string]bool),
	}
}

// ChangeLoader is an interface for loading changes from different sources
type ChangeLoader interface {
	Load() (*ChangeSet, error)
}

// LoaderOptions holds configuration for change loading
type LoaderOptions struct {
	DiffFile string
}

// LoadChanges determines the appropriate ChangeLoader based on options.
// By default (no diff file), all issues in packages are reported.
// Use --diff to filter issues to only changed lines.
func LoadChanges(opts LoaderOptions) (*ChangeSet, error) {
	// If no diff file specified, report all issues (no filtering)
	if opts.DiffFile == "" {
		log.Println("Checking all issues in packages")
		return nil, nil
	}

	log.Printf("Using diff file: %s", opts.DiffFile)
	loader := &DiffFileLoader{filePath: opts.DiffFile}

	cs, err := loader.Load()
	if err != nil {
		return nil, err
	}

	// Set global ChangeSet for package-level functions
	globalChangeSet = cs

	return cs, nil
}

// ShouldReport checks if a specific line in a file should be reported
func ShouldReport(filename string, line int) bool {
	if globalChangeSet == nil {
		return true
	}
	return globalChangeSet.ShouldReport(filename, line)
}

// IsFileChanged checks if a file has any changes
func IsFileChanged(filename string) bool {
	if globalChangeSet == nil {
		return true
	}
	return globalChangeSet.IsFileChanged(filename)
}

// IsNewFile checks if a file is newly added
func IsNewFile(filename string) bool {
	if globalChangeSet == nil {
		return true
	}
	return globalChangeSet.IsNewFile(filename)
}

// IsEnabled checks if change tracking is enabled and has data
func IsEnabled() bool {
	if globalChangeSet == nil {
		return false
	}
	return globalChangeSet.isEnabled()
}

// GetStats returns statistics about tracked changes
func GetStats() (filesCount int, totalLines int) {
	if globalChangeSet == nil {
		return 0, 0
	}
	return globalChangeSet.getStats()
}

// GetChangedPackages returns a list of package paths based on changed files
func GetChangedPackages() []string {
	if globalChangeSet == nil || len(globalChangeSet.changedFiles) == 0 {
		return nil
	}
	return globalChangeSet.getChangedPackages()
}

// ShouldReport checks if a specific line in a file should be reported
func (cs *ChangeSet) ShouldReport(filename string, line int) bool {
	if len(cs.changedLines) == 0 {
		return false
	}

	relPath := normalizeFilePath(filename)
	if !isServiceFile(relPath) {
		return false
	}

	if lineMap, exists := cs.changedLines[relPath]; exists {
		return lineMap[line]
	}

	return false
}

// IsFileChanged checks if a file has any changes
func (cs *ChangeSet) IsFileChanged(filename string) bool {
	if len(cs.changedFiles) == 0 {
		return false
	}

	relPath := normalizeFilePath(filename)
	if !isServiceFile(relPath) {
		return false
	}

	return cs.changedFiles[relPath]
}

// IsNewFile checks if a file is newly added
func (cs *ChangeSet) IsNewFile(filename string) bool {
	if len(cs.newFiles) == 0 {
		return false
	}

	relPath := normalizeFilePath(filename)
	if !isServiceFile(relPath) {
		return false
	}

	return cs.newFiles[relPath]
}

// isEnabled checks if change tracking is enabled and has data
func (cs *ChangeSet) isEnabled() bool {
	return len(cs.changedLines) > 0
}

// getStats returns statistics about tracked changes
func (cs *ChangeSet) getStats() (filesCount int, totalLines int) {
	filesCount = len(cs.changedFiles)
	totalLines = cs.getTotalChangedLines()
	return
}

// getTotalChangedLines counts total changed lines across all files
func (cs *ChangeSet) getTotalChangedLines() int {
	total := 0
	for _, lines := range cs.changedLines {
		total += len(lines)
	}
	return total
}

// getChangedPackages returns a list of unique package paths based on changed files
func (cs *ChangeSet) getChangedPackages() []string {
	packageSet := make(map[string]bool)

	for filePath := range cs.changedFiles {
		// Extract service package path from file path
		// e.g., "internal/services/manageddevopspools/client/client.go" -> "./internal/services/manageddevopspools"
		// e.g., "internal/services/policy/policy_assignment_resource.go" -> "./internal/services/policy"

		if !strings.Contains(filePath, servicePathPrefix) {
			continue
		}

		// Split by service prefix to get the service and subpath
		parts := strings.SplitN(filePath, servicePathPrefix, 2)
		if len(parts) < 2 {
			continue
		}

		// Get the service name (first directory after internal/services/)
		serviceParts := strings.Split(parts[1], "/")
		if len(serviceParts) > 0 {
			serviceName := serviceParts[0]
			packagePath := "./" + servicePathPrefix + serviceName
			packageSet[packagePath] = true
		}
	}

	// Convert map to slice
	packages := make([]string, 0, len(packageSet))
	for pkg := range packageSet {
		packages = append(packages, pkg)
	}

	return packages
}

// parsePatch parses a patch string and extracts changed line numbers into the ChangeSet
func (cs *ChangeSet) parsePatch(filePath string, patchContent string) error {
	scanner := bufio.NewScanner(strings.NewReader(patchContent))
	var currentLine int
	inHunk := false

	// Initialize the map once
	if cs.changedLines[filePath] == nil {
		cs.changedLines[filePath] = make(map[int]bool)
	}

	for scanner.Scan() {
		line := scanner.Text()

		if matches := hunkRegex.FindStringSubmatch(line); matches != nil {
			startLine, err := strconv.Atoi(matches[1])
			if err != nil {
				continue
			}
			currentLine = startLine
			inHunk = true
			continue
		}
		if !inHunk {
			continue
		}

		if len(line) == 0 {
			currentLine++
			continue
		}

		prefix := line[0]
		switch prefix {
		case '+':
			cs.changedLines[filePath][currentLine] = true
			currentLine++
		case ' ':
			currentLine++
		}
	}

	return scanner.Err()
}

// isServiceFile checks if a path is within the service directory
func isServiceFile(path string) bool {
	return strings.Contains(path, servicePathPrefix)
}

// normalizeFilePath normalizes a file path to a consistent format
func normalizeFilePath(filename string) string {
	normalizedFilename := filepath.ToSlash(filename)
	idx := strings.Index(normalizedFilename, servicePathPrefix)
	if idx < 0 {
		return normalizedFilename
	}
	return normalizedFilename[idx:]
}

// parseDiffOutput parses git diff output containing multiple files into the ChangeSet
func (cs *ChangeSet) parseDiffOutput(diffOutput string) error {
	diffGitRegex := regexp.MustCompile(`(?m)^diff --git a/(.+) b/(.+)$`)
	matches := diffGitRegex.FindAllStringSubmatchIndex(diffOutput, -1)
	isNewFileRegex := regexp.MustCompile(`(?m)^new file mode`)

	if len(matches) == 0 {
		return nil // No changes
	}

	for i, match := range matches {
		// Extract file path from the match (use b/ path which is the new path)
		fileName := diffOutput[match[4]:match[5]]

		if !isServiceFile(fileName) {
			continue
		}

		// Get the content of this file's diff (from this match to the next, or to the end)
		var patchContent string
		if i < len(matches)-1 {
			patchContent = diffOutput[match[0]:matches[i+1][0]]
		} else {
			patchContent = diffOutput[match[0]:]
		}

		normalizedPath := normalizeFilePath(fileName)

		isNewFile := isNewFileRegex.MatchString(patchContent)

		if err := cs.parsePatch(normalizedPath, patchContent); err != nil {
			log.Printf("Warning: failed to parse patch for %s: %v", normalizedPath, err)
			continue
		}

		cs.changedFiles[normalizedPath] = true
		if isNewFile {
			cs.newFiles[normalizedPath] = true
		}
	}

	return nil
}
