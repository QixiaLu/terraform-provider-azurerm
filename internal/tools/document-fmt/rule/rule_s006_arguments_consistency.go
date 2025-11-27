// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/markdown"
)

// S006 performs all arguments-related checks in a single pass
// This merges the functionality of existence, requiredness, defaults, and ForceNew checks
// Uses composition to delegate checks to specialized checkers
type S006 struct{}

var _ Rule = new(S006)

func (s S006) ID() string {
	return "S006"
}

func (s S006) Name() string {
	return "Arguments Properties Check"
}

func (s S006) Description() string {
	return "Validates all properties in Arguments section including existence, requiredness, defaults, and ForceNew markers"
}

func (s S006) Run(d *data.TerraformNodeData, fix bool) []error {
	if !d.Document.Exists {
		return nil
	}

	if d.Type == data.ResourceTypeData {
		return nil
	}

	if d.SchemaProperties == nil || d.DocumentArguments == nil {
		return nil
	}

	// Use ExistenceChecker for property existence validation
	existenceChecker := &ExistenceChecker{}

	var issues []*CheckIssue
	resourceType := d.Name

	// First pass: check schema properties for existence
	issues = append(issues, existenceChecker.CheckMissingInDoc(
		d, "", d.SchemaProperties, d.DocumentArguments,
		d.DocumentArguments.BlockDefinitions, resourceType)...)

	// Second pass: check for marker consistency (requiredness, forcenew) recursively
	issues = append(issues, s.checkMarkersInDoc(
		"", d.SchemaProperties, d.DocumentArguments,
		d.DocumentArguments.BlockDefinitions, resourceType)...)

	// Third pass: check orphaned doc properties
	issues = append(issues, existenceChecker.CheckMissingInSchema(
		"", d.DocumentArguments, d.SchemaProperties,
		d.DocumentArguments.BlockDefinitions, resourceType)...)

	// Merge potential misspellings issues
	issues = existenceChecker.mergeMisspellings(issues)

	if fix {
		s.applyFixes(d, issues)
		return nil
	}

	var errs []error
	for _, issue := range issues {
		errs = append(errs, issue)
	}
	return errs
}

// checkMarkersInDoc recursively checks marker consistency (requiredness, forcenew) for properties
// that exist in both schema and documentation
func (s S006) checkMarkersInDoc(
	parentPath string,
	schema *models.SchemaProperties,
	documentation *models.DocumentProperties,
	blockDefinitions map[string]*models.DocumentProperty,
	resourceType string,
) []*CheckIssue {
	var issues []*CheckIssue

	if schema == nil || documentation == nil {
		return issues
	}

	for name, schemaProperty := range schema.Objects {
		// Skip computed-only properties and 'id' field
		if !schemaProperty.Optional && schemaProperty.Computed {
			continue
		}
		if name == "id" {
			continue
		}
		// Skip deprecated properties
		if schemaProperty.Deprecated {
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

		docProperty := documentation.Objects[name]
		if docProperty == nil {
			// Property doesn't exist in documentation, skip marker checks
			continue
		}

		// Check for block type declarations (nested properties)
		if schemaProperty.Nested != nil && len(schemaProperty.Nested.Objects) > 0 {
			if !docProperty.Block {
				// Block declaration error already reported in existence check
				continue
			}

			if docProperty.Nested == nil || len(docProperty.Nested.Objects) == 0 {
				// For some blocks sharing same sub-fields, they are defined in a shared block section
				if docProperty.BlockTypeName != docProperty.Name {
					linkedDocProperty := blockDefinitions[docProperty.BlockTypeName]
					if linkedDocProperty != nil && linkedDocProperty.Nested != nil && len(linkedDocProperty.Nested.Objects) > 0 {
						// Recursively check marker issues for shared block's nested properties
						issues = append(issues, s.checkMarkersInDoc(fullPath, schemaProperty.Nested, linkedDocProperty.Nested, blockDefinitions, resourceType)...)
						// Check this block's own markers
						markerIssues := s.checkSchemaProperty(fullPath, schemaProperty, docProperty, blockDefinitions)
						issues = append(issues, markerIssues...)
						continue
					}
				}

				continue
			}

			// Recursively check marker issues for nested properties
			if docProperty.Nested != nil {
				issues = append(issues, s.checkMarkersInDoc(fullPath, schemaProperty.Nested, docProperty.Nested, blockDefinitions, resourceType)...)
			}

			// Check this block's own markers (Requiredness, ForceNew)
			markerIssues := s.checkSchemaProperty(fullPath, schemaProperty, docProperty, blockDefinitions)
			issues = append(issues, markerIssues...)
		} else {
			// For non-nested properties: check markers (Requiredness, ForceNew)
			markerIssues := s.checkSchemaProperty(fullPath, schemaProperty, docProperty, blockDefinitions)
			issues = append(issues, markerIssues...)
		}
	}

	return issues
}

func (s S006) checkSchemaProperty(
	fullPath string,
	schemaProperty *models.SchemaProperty,
	docProperty *models.DocumentProperty,
	blockDefinitions map[string]*models.DocumentProperty,
) []*CheckIssue {
	var issues []*CheckIssue

	if docProperty == nil || schemaProperty == nil {
		return issues
	}

	ctx := &PropertyCheckContext{
		FullPath:         fullPath,
		SchemaProperty:   schemaProperty,
		DocProperty:      docProperty,
		BlockDefinitions: blockDefinitions,
	}

	// Check requiredness markers (Required/Optional)
	requiredChecker := &RequirednesChecker{}
	issues = append(issues, requiredChecker.CheckMarkers(ctx)...)

	// Check forcenew markers
	forceNewChecker := &ForceNewChecker{}
	issues = append(issues, forceNewChecker.CheckMarkers(ctx)...)

	return issues
}

// applyFixes applies all issue fixes to the document
func (s S006) applyFixes(d *data.TerraformNodeData, issues []*CheckIssue) {
	if d.Document == nil || len(issues) == 0 {
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

	// Group issues by line number to handle multiple fixes on the same line
	issuesByLine := make(map[int][]*CheckIssue)
	for _, issue := range issues {
		if issue.FixLine == "" {
			continue
		}
		lineIdx := issue.LineNum
		if lineIdx >= 0 && lineIdx < len(content) {
			issuesByLine[lineIdx] = append(issuesByLine[lineIdx], issue)
		}
	}

	// Use checker instances for fixes
	requiredChecker := &RequirednesChecker{}
	forceNewChecker := &ForceNewChecker{}

	// Apply fixes for each line
	for lineIdx, lineIssues := range issuesByLine {
		fixedLine := content[lineIdx]

		// Apply all fixes for this line in sequence
		for _, issue := range lineIssues {
			switch issue.CheckType {
			case "RequiredMiss":
				fixedLine = requiredChecker.FixRequiredness(fixedLine, "(Optional)", "(Required)")
			case "OptionalMiss":
				fixedLine = requiredChecker.FixRequiredness(fixedLine, "(Required)", "(Optional)")
			}
		}

		for _, issue := range lineIssues {
			switch issue.CheckType {
			case "ForceNewMiss":
				shouldAdd := strings.Contains(issue.Message, "should be marked as ForceNew")
				fixedLine = forceNewChecker.FixForceNew(fixedLine, shouldAdd)
			}
		}

		content[lineIdx] = fixedLine
	}

	argsSection.SetContent(content)
	d.Document.HasChange = true
}
