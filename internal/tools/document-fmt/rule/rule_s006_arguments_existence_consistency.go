// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
)

// ExistenceChecker performs property existence and block declaration checks
type ExistenceChecker struct{}

// CheckMissingInDoc checks if schema properties are documented and validates block declarations
// Recursively processes nested properties
func (ec *ExistenceChecker) CheckMissingInDoc(
	d *data.TerraformNodeData,
	parentPath string,
	schema *models.SchemaProperties,
	documentation *models.DocumentProperties,
	blockDefinitions map[string]*models.DocumentProperty,
	resourceType string,
	fileName string,
) []*CheckIssue {
	var issues []*CheckIssue

	if schema == nil {
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

		// Check if property exists in documentation
		docProperty := documentation.Objects[name]
		if docProperty == nil {
			issues = append(issues, &CheckIssue{
				LineNum:   0,
				Key:       fullPath,
				Message:   fmt.Sprintf("S006: %s: `%s` exists in schema but is missing from documentation", fileName, fullPath),
				CheckType: "MissInDoc",
			})
			continue
		}

		// Check for block type declarations (nested properties)
		if schemaProperty.Nested != nil && len(schemaProperty.Nested.Objects) > 0 {
			// Check if the field is marked as a block in documentation
			if !docProperty.Block {
				issues = append(issues, &CheckIssue{
					LineNum:   docProperty.Line,
					Key:       fullPath,
					Message:   fmt.Sprintf("S006: %s: `%s` should be declared as a block", fileName, fullPath),
					CheckType: "MissBlockDeclare",
				})
				continue
			}

			if docProperty.Nested == nil || len(docProperty.Nested.Objects) == 0 {
				// For some blocks sharing same sub-fields, they are defined in a shared block section
				if docProperty.BlockTypeName != docProperty.Name {
					linkedDocProperty := blockDefinitions[docProperty.BlockTypeName]
					if linkedDocProperty != nil && linkedDocProperty.Nested != nil && len(linkedDocProperty.Nested.Objects) > 0 {
						// Recursively check nested properties in shared block
						issues = append(issues, ec.CheckMissingInDoc(d, fullPath, schemaProperty.Nested, linkedDocProperty.Nested, blockDefinitions, resourceType, fileName)...)
						continue
					}
				}

				issues = append(issues, &CheckIssue{
					LineNum:   docProperty.Line,
					Key:       fullPath,
					Message:   fmt.Sprintf("S006: %s: `%s` block is missing from documentation", fileName, fullPath),
					CheckType: "MissBlockDeclare",
				})
				continue
			}

			// Recursively check nested properties
			if docProperty.Nested != nil {
				issues = append(issues, ec.CheckMissingInDoc(d, fullPath, schemaProperty.Nested, docProperty.Nested, blockDefinitions, resourceType, fileName)...)
			}
		}
	}

	return issues
}

// CheckMissingInSchema checks if documented properties are missing from schema (orphaned properties)
// Recursively processes nested properties
func (ec *ExistenceChecker) CheckMissingInSchema(
	parentPath string,
	documentation *models.DocumentProperties,
	schema *models.SchemaProperties,
	blockDefinitions map[string]*models.DocumentProperty,
	resourceType string,
	fileName string,
) []*CheckIssue {
	var issues []*CheckIssue

	if documentation == nil {
		return issues
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

		schemaProperty := schema.Objects[name]

		if len(docProperty.ParseErrors) > 0 {
			for _, parseErr := range docProperty.ParseErrors {
				if strings.Contains(parseErr, "misspell of name from") {
					issues = append(issues, &CheckIssue{
						LineNum:   docProperty.Line,
						Key:       fullPath,
						Message:   fmt.Sprintf("S006: %s: `%s` does not exist in schema - possible misspelling?", fileName, fullPath),
						DocProp:   docProperty,
						CheckType: "Misspelling",
					})
				}
			}
			continue
		}

		// Check for deprecated properties in documentation content
		if strings.Contains(strings.ToLower(docProperty.Content), "deprecated") {
			continue
		}

		// If schema property doesn't exist, check orphaned property
		if schemaProperty == nil {
			issues = append(issues, &CheckIssue{
				LineNum:   docProperty.Line,
				Key:       fullPath,
				Message:   fmt.Sprintf("S006: %s: `%s` exists in documentation but not in schema", fileName, fullPath),
				DocProp:   docProperty,
				CheckType: "MissInCode",
			})
			continue
		}

		// Check if document marks field as block but schema doesn't have nested properties
		if docProperty.Block && (schemaProperty.Nested == nil || len(schemaProperty.Nested.Objects) == 0) {
			issues = append(issues, &CheckIssue{
				LineNum:   docProperty.Line,
				Key:       fullPath,
				Message:   fmt.Sprintf("S006: %s: The document incorrectly implies `%s` is a block (contains phrases like 'as defined below')", fileName, fullPath),
				DocProp:   docProperty,
				CheckType: "IncorrectlyBlockMarked",
			})
			continue
		}

		// Recursively check nested orphaned properties
		if docProperty.Nested != nil && len(docProperty.Nested.Objects) > 0 {
			if schemaProperty.Nested != nil {
				issues = append(issues, ec.CheckMissingInSchema(fullPath, docProperty.Nested, schemaProperty.Nested, blockDefinitions, resourceType, fileName)...)
			}
		}
	}

	return issues
}

// mergeMisspellings identifies potential misspellings by comparing missed properties
func (ec *ExistenceChecker) mergeMisspellings(issues []*CheckIssue, fileName string) []*CheckIssue {
	var missInDoc, missInCode []*CheckIssue

	// Collect all miss-in-doc and miss-in-code issues
	for _, issue := range issues {
		if issue.CheckType == "MissInDoc" {
			missInDoc = append(missInDoc, issue)
		} else if issue.CheckType == "MissInCode" {
			missInCode = append(missInCode, issue)
		}
	}

	// Find potential misspellings using Levenshtein distance
	filterOut := make(map[*CheckIssue]struct{})
	var misspellings []*CheckIssue

	for _, codeIssue := range missInCode {
		for _, docIssue := range missInDoc {
			codeName := extractBaseName(codeIssue.Key)
			docName := extractBaseName(docIssue.Key)

			if dist := levenshteinDist(codeName, docName); dist <= 3 {
				filterOut[codeIssue] = struct{}{}
				filterOut[docIssue] = struct{}{}

				misspellings = append(misspellings, &CheckIssue{
					LineNum:   codeIssue.LineNum,
					Key:       codeIssue.Key,
					Message:   fmt.Sprintf("S006: %s: `%s` does not exist in schema - should this be `%s`?", fileName, codeName, docName),
					DocProp:   codeIssue.DocProp,
					CheckType: "Misspelling",
				})
			}
		}
	}

	// Rebuild issue list
	var result []*CheckIssue
	for _, issue := range issues {
		if _, shouldFilter := filterOut[issue]; !shouldFilter {
			result = append(result, issue)
		}
	}
	result = append(result, misspellings...)

	return result
}

// levenshteinDist calculates the Levenshtein distance between two strings
// Used for detecting potential misspellings
func levenshteinDist(str1, str2 string) int {
	column := make([]int, len(str1)+1)
	for y := 1; y <= len(str1); y++ {
		column[y] = y
	}

	for x := 1; x <= len(str2); x++ {
		column[0] = x
		lastKey := x - 1
		for y := 1; y <= len(str1); y++ {
			oldKey := column[y]
			incr := 0
			if str1[y-1] != str2[x-1] {
				incr = 1
			}

			column[y] = minimumOf3(column[y]+1, column[y-1]+1, lastKey+incr)
			lastKey = oldKey
		}
	}
	return column[len(str1)]
}

func minimumOf3(a, b, c int) int {
	if a > b {
		a = b
	}
	if a > c {
		a = c
	}
	return a
}

// extractBaseName extracts the base name from a property path
func extractBaseName(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
