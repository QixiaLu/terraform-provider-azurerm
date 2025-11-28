// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/markdown"
)

// S008 validates and fixes ForceNew markers in documentation match schema
type S008 struct{}

var _ Rule = new(S008)

var forceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]*(\.|$)`)

func (s S008) ID() string {
	return "S008"
}

func (s S008) Name() string {
	return "Arguments ForceNew Consistency"
}

func (s S008) Description() string {
	return "Validates that ForceNew markers in documentation match schema definition"
}

func (s S008) Run(d *data.TerraformNodeData, fix bool) []error {
	if !d.Document.Exists {
		return nil
	}

	if d.Type == data.ResourceTypeData {
		return nil
	}

	if d.SchemaProperties == nil || d.DocumentArguments == nil {
		return nil
	}

	var errs []error
	resourceType := d.Name

	// Check ForceNew markers recursively
	errs = append(errs, s.checkForceNew(d, "", d.SchemaProperties, d.DocumentArguments, d.DocumentArguments.BlockDefinitions, resourceType, fix)...)

	return errs
}

// checkForceNew recursively checks ForceNew markers for properties that exist in both schema and documentation
func (s S008) checkForceNew(
	d *data.TerraformNodeData,
	parentPath string,
	schema *models.SchemaProperties,
	documentation *models.DocumentProperties,
	blockDefinitions map[string]*models.DocumentProperty,
	resourceType string,
	fix bool,
) []error {
	var errs []error

	if schema == nil || documentation == nil {
		return errs
	}

	for name, schemaProperty := range schema.Objects {
		if !schemaProperty.Optional && schemaProperty.Computed {
			continue
		}
		if name == "id" {
			continue
		}
		if schemaProperty.Deprecated {
			continue
		}

		fullPath := name
		if parentPath != "" {
			fullPath = parentPath + "." + name
		}

		if isSkipProp(resourceType, fullPath) {
			continue
		}

		docProperty := documentation.Objects[name]
		if docProperty == nil {
			continue
		}

		if len(docProperty.ParseErrors) > 0 {
			continue
		}

		if schemaProperty.Nested != nil && len(schemaProperty.Nested.Objects) > 0 {
			if !docProperty.Block {
				// Block declaration error, skip
				continue
			}

			if docProperty.Nested == nil || len(docProperty.Nested.Objects) == 0 {
				// For some blocks sharing same sub-fields, they are defined in a shared block section
				if docProperty.BlockTypeName != docProperty.Name {
					linkedDocProperty := blockDefinitions[docProperty.BlockTypeName]
					if linkedDocProperty != nil && linkedDocProperty.Nested != nil && len(linkedDocProperty.Nested.Objects) > 0 {
						// Recursively check nested properties in shared block
						errs = append(errs, s.checkForceNew(d, fullPath, schemaProperty.Nested, linkedDocProperty.Nested, blockDefinitions, resourceType, fix)...)
						// Check this block's own ForceNew marker
						errs = append(errs, s.checkPropertyForceNew(d, fullPath, schemaProperty, docProperty, fix)...)
						continue
					}
				}

				continue
			}

			// Recursively check nested properties
			if docProperty.Nested != nil {
				errs = append(errs, s.checkForceNew(d, fullPath, schemaProperty.Nested, docProperty.Nested, blockDefinitions, resourceType, fix)...)
			}

			// Check this block's own ForceNew marker
			errs = append(errs, s.checkPropertyForceNew(d, fullPath, schemaProperty, docProperty, fix)...)
		} else {
			// For non-nested properties: check ForceNew marker
			errs = append(errs, s.checkPropertyForceNew(d, fullPath, schemaProperty, docProperty, fix)...)
		}
	}

	return errs
}

// checkPropertyForceNew checks and optionally fixes ForceNew marker for a single property
func (s S008) checkPropertyForceNew(
	d *data.TerraformNodeData,
	fullPath string,
	schemaProperty *models.SchemaProperty,
	docProperty *models.DocumentProperty,
	fix bool,
) []error {
	var errs []error

	if docProperty == nil || schemaProperty == nil {
		return errs
	}

	// Skip resource_group_name as per existing logic
	if lastPathSegment(fullPath) == "resource_group_name" {
		return errs
	}

	// Check: ForceNew markers
	if schemaProperty.ForceNew != docProperty.ForceNew {
		if schemaProperty.ForceNew && !docProperty.ForceNew {
			// Should add ForceNew marker
			origLine := strings.TrimRight(docProperty.Content, "\n")
			fixedLine := s.fixForceNew(origLine, true)
			issue := NewValidationIssue(
				s.ID(),
				s.Name(),
				fullPath,
				fmt.Sprintf("`%s` should be marked as ForceNew", fullPath),
				d.Document.Path,
				origLine,
				fixedLine,
			)
			errs = append(errs, issue)

			if fix {
				s.applyForceNewFix(d, docProperty, true)
			}
		} else if docProperty.ForceNew && !schemaProperty.ForceNew {
			// Should remove ForceNew marker
			origLine := strings.TrimRight(docProperty.Content, "\n")
			fixedLine := s.fixForceNew(origLine, false)
			issue := NewValidationIssue(
				s.ID(),
				s.Name(),
				fullPath,
				fmt.Sprintf("`%s` should not be marked as ForceNew", fullPath),
				d.Document.Path,
				origLine,
				fixedLine,
			)
			errs = append(errs, issue)

			if fix {
				s.applyForceNewFix(d, docProperty, false)
			}
		}
	}

	return errs
}

// applyForceNewFix applies ForceNew fix to the document
func (s S008) applyForceNewFix(d *data.TerraformNodeData, docProperty *models.DocumentProperty, shouldAdd bool) {
	if d.Document == nil {
		return
	}

	// Find Arguments section
	var argsSection markdown.Section
	for _, section := range d.Document.Sections {
		if _, ok := section.(*markdown.ArgumentsSection); ok {
			argsSection = section
			break
		}
	}

	if argsSection == nil {
		return
	}

	content := argsSection.GetContent()
	lineIdx := docProperty.Line

	if lineIdx >= 0 && lineIdx < len(content) {
		line := content[lineIdx]
		fixedLine := s.fixForceNew(line, shouldAdd)
		content[lineIdx] = fixedLine
		argsSection.SetContent(content)
		d.Document.HasChange = true
	}
}

func (s S008) fixForceNew(line string, shouldAdd bool) string {
	if shouldAdd {
		// Add ForceNew message if not present
		line = strings.TrimRight(line, " \t\r\n")
		if strings.HasSuffix(line, ",") {
			line = line[:len(line)-1] + "."
		} else if !strings.HasSuffix(line, ".") {
			line += "."
		}
		line += " Changing this forces a new resource to be created."
	} else {
		// Remove ForceNew message
		line = forceNewReg.ReplaceAllString(line, "")
	}
	return line
}

// lastPathSegment extracts the last segment of a dotted path
func lastPathSegment(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
