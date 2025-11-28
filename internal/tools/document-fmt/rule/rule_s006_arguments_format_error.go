// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/mdparser"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
)

// FormatErrorChecker checks for document formatting errors
type FormatErrorChecker struct{}

var regIncorrectBlockMark = regexp.MustCompile(`(?: blocks?)? as (?:detailed|defined) (below|above)`)

// CheckFormatError checks for formatting errors in a single documented property
func (fec *FormatErrorChecker) CheckFormatError(
	fullPath string,
	docProperty *models.DocumentProperty,
	schemaProperty *models.SchemaProperty,
	fileName string,
) []*CheckIssue {
	var issues []*CheckIssue

	if docProperty == nil || len(docProperty.ParseErrors) == 0 {
		return issues
	}

	// Handle parse errors
	for _, parseErr := range docProperty.ParseErrors {
		// Check if "block is not defined" error should be converted to "incorrectly block marked"
		if strings.Contains(parseErr, mdparser.BlcokNotDefined) && schemaProperty != nil {
			// If schema property exists but is not a block, update the error type
			if schemaProperty.Nested == nil || len(schemaProperty.Nested.Objects) == 0 {
				parseErr = "incorrectly block marked"
			}
		}

		if strings.Contains(parseErr, "misspell of name from") {
			continue
		}

		var message string
		if strings.Contains(parseErr, mdparser.IncorrectlyBlockMarked) {
			message = fmt.Sprintf("S006: %s: The document incorrectly implies `%s` is a block (contains phrases like 'as defined below')", fileName, fullPath)
		// } else if strings.TrimSpace(docProperty.Content) == "*" {
		// 	message = fmt.Sprintf("S006: %s: Found a list marker with no field name or content. This should be removed", fileName)
		// } else if strings.HasPrefix(docProperty.Content, "* ~>") {
		// 	message = fmt.Sprintf("S006: %s: a Note block should not start with `*`", fileName)   // Comments these out, cause the line number is relevant to section, not to the whole doc
		} else if strings.Contains(parseErr, "duplicate") {
			message = fmt.Sprintf("S006: %s: %s: `%s`", fileName, parseErr, fullPath)
		} else if strings.Contains(parseErr, "no field name found") {
			message = fmt.Sprintf("S006: %s: following should be formatted as: `* `field` - (Required/Optional) Xxx...`\n %s", fileName, docProperty.Content)
		} else {
			message = fmt.Sprintf("S006: %s: %s", fileName, parseErr)
		}

		issues = append(issues, &CheckIssue{
			LineNum:   docProperty.Line,
			Key:       fullPath,
			Message:   message,
			DocProp:   docProperty,
			FixLine:   docProperty.Content,
			CheckType: "FormatError",
		})
	}

	return issues
}

// FixFormatError applies formatting fixes to a line
func (fec *FormatErrorChecker) FixFormatError(line string, issue *CheckIssue) string {
	if strings.HasPrefix(line, "* ~>") {
		// Remove misleading star mark from Note lines
		return strings.TrimPrefix(line, "* ")
	}

	if strings.TrimSpace(line) == "*" {
		// Remove empty list markers
		return ""
	}

	if strings.Contains(issue.Message, "incorrectly implies") && strings.Contains(issue.Message, "is a block") {
		// Remove incorrect block markers using regex
		return regIncorrectBlockMark.ReplaceAllLiteralString(line, "")
	}

	return line
}
