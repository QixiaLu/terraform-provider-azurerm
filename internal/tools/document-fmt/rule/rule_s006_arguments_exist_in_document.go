package rule

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
)

type S006 struct{}

var _ Rule = new(S006)

func (s S006) ID() string {
	return "S006"
}

func (s S006) Name() string {
	return "Arguments Exist in Document"
}

func (s S006) Description() string {
	return "Determines whether all arguments defined in schema are documented, and checks for missing/misspelled properties"
}

func (s S006) Run(d *data.TerraformNodeData, fix bool) []error {
	var errs []error

	if !d.Document.Exists {
		return errs
	}

	// TODO: complete data source check
	if d.Type == data.ResourceTypeData {
		return errs
	}

	// If either is nil, we can't properly check, so short-circuit
	if d.SchemaProperties == nil || d.DocumentArguments == nil {
		return errs
	}

	// Check for properties missing in documentation
	errs = append(errs, s.checkMissingInDoc(d.Name, "", d.SchemaProperties, d.DocumentArguments)...)

	// Check for properties missing in schema (might be typos or deprecated)
	errs = append(errs, s.checkMissingInSchema(d.Name, "", d.DocumentArguments, d.SchemaProperties)...)

	return errs
}

// checkMissingInDoc checks if schema properties are missing from documentation
func (s S006) checkMissingInDoc(resourceType, parentPath string, schema *data.Properties, documentation *data.Properties) []error {
	errs := make([]error, 0)

	if schema == nil {
		return errs
	}

	for name, property := range schema.Objects {
		// Skip computed-only properties and 'id' field
		if !property.Optional && property.Computed {
			continue
		}
		if name == "id" {
			continue
		}
		// Skip deprecated properties
		if property.Deprecated {
			continue
		}

		fullPath := name
		if parentPath != "" {
			fullPath = parentPath + "." + name
		}

		// Check if property exists in documentation
		docProperty := documentation.Objects[name]
		if docProperty == nil {
			errs = append(errs, fmt.Errorf("%s: argument `%s` exists in schema but is missing from documentation at line %d",
				IdAndName(s), fullPath, 0))
			continue
		}

		// Check for block type declarations
		if property.Nested != nil && len(property.Nested.Objects) > 0 {
			if docProperty.Nested == nil || len(docProperty.Nested.Objects) == 0 {
				// Check if the field is marked as a block in documentation
				if !docProperty.Block {
					errs = append(errs, fmt.Errorf("%s: argument `%s` should be declared as a block (e.g., 'One or more `%s` block as defined below') at line %d",
						IdAndName(s), fullPath, name, docProperty.Line))
					continue
				}

				errs = append(errs, fmt.Errorf("%s: `%s` block is missing from documentation (e.g. A / An `%s` block supports the following:)"))
				continue
			}

			// Recursively check nested properties
			if docProperty.Nested != nil {
				errs = append(errs, s.checkMissingInDoc(resourceType, fullPath, property.Nested, docProperty.Nested)...)
			}
		}
	}

	return errs
}

// checkMissingInSchema checks if documented properties are missing from schema
func (s S006) checkMissingInSchema(resourceType, parentPath string, documentation *data.Properties, schema *data.Properties) []error {
	errs := make([]error, 0)

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

		// Skip block definition sections (these are documentation sections, not actual properties)
		// A block definition has Block=true AND populated Nested properties
		// These are standalone sections like "A `restore_policy` block supports:"
		if parentPath == "" && docProperty.Block && docProperty.Nested != nil && len(docProperty.Nested.Objects) > 0 {
			// This is a block definition section at the root level - skip it
			// The actual block field references are checked when processing parent blocks
			continue
		}

		// Check if documentation mentions deprecated
		if strings.Contains(strings.ToLower(docProperty.Content), "deprecated") {
			continue
		}

		// Check if schema property exists
		schemaProperty := schema.Objects[name]
		if schemaProperty == nil {
			// Check for "not available for" pattern
			if idx := strings.Index(strings.ToLower(docProperty.Content), "not available for"); idx > 0 {
				// Extract the code value after "not available for"
				remaining := docProperty.Content[idx:]
				if codeValue := firstCodeValue(remaining); codeValue != "" && strings.Contains(fullPath, codeValue) {
					continue
				}
			}

			errs = append(errs, fmt.Errorf("%s: argument `%s` is documented at line %d but does not exist in schema - should this be removed or is it misspelled?",
				IdAndName(s), fullPath, docProperty.Line))
			continue
		}

		// Recursively check nested properties
		// Skip if this property is marked as Block - its nested properties are documented separately
		if docProperty.Block {
			continue
		}

		if docProperty.Nested != nil && len(docProperty.Nested.Objects) > 0 {
			if schemaProperty.Nested != nil {
				errs = append(errs, s.checkMissingInSchema(resourceType, fullPath, docProperty.Nested, schemaProperty.Nested)...)
			}
		}
	}

	return errs
}

// firstCodeValue extracts the first code value (text in backticks) from a string
func firstCodeValue(text string) string {
	start := strings.Index(text, "`")
	if start == -1 {
		return ""
	}
	end := strings.Index(text[start+1:], "`")
	if end == -1 {
		return ""
	}
	return text[start+1 : start+1+end]
}
