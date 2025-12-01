// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"path/filepath"
	"strings"
)

// ValidationIssue represents a validation issue with before/after comparison
type ValidationIssue struct {
	RuleID      string
	RuleName    string
	PropertyKey string
	Message     string
	FileName    string
	OrigLine    string // Original line content
	FixedLine   string // Fixed line content (empty if no fix available)
}

func (vi *ValidationIssue) Error() string {
	var result strings.Builder

	// Format: "S007: fileName: message"
	result.WriteString(vi.RuleID)
	result.WriteString(": ")
	result.WriteString(getRelevantPath(vi.FileName))
	result.WriteString(": ")
	result.WriteString(vi.Message)

	// If we have both original and fixed lines, show the comparison
	if vi.OrigLine != "" && vi.FixedLine != "" && vi.OrigLine != vi.FixedLine {
		result.WriteString("\n     ")
		result.WriteString(strings.TrimRight(vi.OrigLine, "\n"))
		result.WriteString("\n  => ")
		result.WriteString(strings.TrimRight(vi.FixedLine, "\n"))
		result.WriteString("\n")
	}

	return result.String()
}

// NewValidationIssue creates a new validation issue
func NewValidationIssue(ruleID, ruleName, propertyKey, message, fileName, origLine, fixedLine string) *ValidationIssue {
	return &ValidationIssue{
		RuleID:      ruleID,
		RuleName:    ruleName,
		PropertyKey: propertyKey,
		Message:     message,
		FileName:    fileName,
		OrigLine:    origLine,
		FixedLine:   fixedLine,
	}
}

// getRelevantPath extracts the relevant part of file path for display
func getRelevantPath(fullPath string) string {
	path := filepath.ToSlash(fullPath)

	if idx := strings.Index(path, "website/docs/"); idx >= 0 {
		return path[idx:]
	}

	return filepath.Base(fullPath)
}
