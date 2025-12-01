// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/mdparser"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
)

// S006 validates and fixes Arguments format errors
type S006 struct{}

var _ Rule = new(S006)

var regIncorrectBlockMark = regexp.MustCompile(`(?: blocks?)? as (?:detailed|defined) (below|above)`)

func (s S006) ID() string {
	return "S006"
}

func (s S006) Name() string {
	return "Arguments Format Errors"
}

func (s S006) Description() string {
	return "Validates and fixes document formatting errors in Arguments section"
}

func (s S006) Run(d *data.TerraformNodeData, fix bool) []error {
	if !d.Document.Exists {
		return nil
	}

	if d.Type == data.ResourceTypeData {
		return nil
	}

	if d.DocumentArguments == nil {
		return nil
	}

	var errs []error
	resourceType := d.Name

	// Check format errors recursively
	errs = append(errs, s.checkFormatErrors(d, "", d.DocumentArguments, d.SchemaProperties, resourceType, fix)...)

	return errs
}

// checkFormatErrors recursively checks for formatting errors in documented properties
func (s S006) checkFormatErrors(
	d *data.TerraformNodeData,
	parentPath string,
	documentation *models.DocumentProperties,
	schema *models.SchemaProperties,
	resourceType string,
	fix bool,
) []error {
	var errs []error

	if documentation == nil {
		return errs
	}

	for name, docProperty := range documentation.Objects {
		// Skip 'id' field
		if name == "id" {
			continue
		}

		fullPath := name
		if parentPath != "" {
			fullPath = parentPath + "." + name
		}

		// Skip properties in skip config
		if isSkipProp(resourceType, fullPath) {
			continue
		}

		var schemaProperty *models.SchemaProperty
		if schema != nil {
			schemaProperty = schema.Objects[name]
		}

		// Handle parse errors
		if len(docProperty.ParseErrors) > 0 {
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
				origLine := strings.TrimRight(docProperty.Content, "\n")
				fixedLine := s.getFixedLine(docProperty, parseErr)

				switch {
				case strings.Contains(parseErr, mdparser.IncorrectlyBlockMarked):
					message = fmt.Sprintf("The document incorrectly implies `%s` is a block (contains phrases like 'as defined below')", fullPath)
				case strings.Contains(parseErr, "duplicate"):
					message = fmt.Sprintf("%s: `%s`", parseErr, fullPath)
				case strings.Contains(parseErr, "no field name found"):
					message = fmt.Sprintf("following should be formatted as: `* `field` - (Required/Optional) Xxx...`\n  %s\n", docProperty.Content)
				default:
					message = parseErr
				}

				issue := NewValidationIssue(
					s.ID(),
					s.Name(),
					fullPath,
					message,
					d.Document.Path,
					origLine,
					fixedLine,
				)
				errs = append(errs, issue)
				if fix {
					s.applyFormatFix(d, docProperty, parseErr)
				}
			}
		}

		// Recursively check nested properties for format errors
		if docProperty.Nested != nil && len(docProperty.Nested.Objects) > 0 {
			var nestedSchema *models.SchemaProperties
			if schemaProperty != nil {
				nestedSchema = schemaProperty.Nested
			}
			errs = append(errs, s.checkFormatErrors(d, fullPath, docProperty.Nested, nestedSchema, resourceType, fix)...)
		}
	}

	return errs
}

// applyFormatFix applies format fix to the document
func (s S006) applyFormatFix(d *data.TerraformNodeData, docProperty *models.DocumentProperty, parseErr string) {
	if d.Document == nil {
		return
	}

	argsSection := d.Document.GetArgumentsSection()
	if argsSection == nil {
		return
	}

	content := argsSection.GetContent()
	lineIdx := docProperty.Line

	if lineIdx >= 0 && lineIdx < len(content) {
		line := content[lineIdx]
		fixedLine := s.fixFormatError(line, parseErr)

		// Note: We don't delete lines to avoid index invalidation for subsequent rules
		// Empty lines will be preserved to maintain line numbers
		if fixedLine != line {
			content[lineIdx] = fixedLine
			argsSection.SetContent(content)
			d.Document.HasChange = true
		}
	}
}

// getFixedLine returns the fixed version of the line for display purposes
func (s S006) getFixedLine(docProperty *models.DocumentProperty, parseErr string) string {
	if docProperty == nil {
		return ""
	}
	line := strings.TrimRight(docProperty.Content, "\n")
	return s.fixFormatError(line, parseErr)
}

// fixFormatError applies formatting fixes to a line based on the parse error
func (s S006) fixFormatError(line string, parseErr string) string {
	// Remove misleading star mark from Note lines
	if strings.HasPrefix(line, "* ~>") {
		return strings.TrimPrefix(line, "* ")
	}

	// Mark empty list markers as empty (but preserve line to avoid index issues)
	if strings.TrimSpace(line) == "*" {
		return "" // Return empty string, line will be cleared but not removed
	}

	// Remove incorrect block markers
	if strings.Contains(parseErr, mdparser.IncorrectlyBlockMarked) {
		return regIncorrectBlockMark.ReplaceAllLiteralString(line, "")
	}

	return line
}
