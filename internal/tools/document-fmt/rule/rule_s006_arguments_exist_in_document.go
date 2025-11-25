package rule

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/util"
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

	if d.SchemaProperties == nil || d.DocumentArguments == nil {
		return errs
	}

	errs = append(errs, s.checkMissingInDoc(d.Name, "", d.SchemaProperties, d.DocumentArguments, d.DocumentArguments.BlockDefinitions)...)
	errs = append(errs, s.checkMissingInSchema(d.Name, "", d.DocumentArguments, d.SchemaProperties, d.DocumentArguments.BlockDefinitions)...)

	return errs
}

// checkMissingInDoc checks if schema properties are missing from documentation
func (s S006) checkMissingInDoc(resourceType, parentPath string, schema *models.SchemaProperties, documentation *models.DocumentProperties, blockDefinitions map[string]*models.DocumentProperty) []error {
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

		// Skip properties in skip config
		if isSkipProp(resourceType, fullPath) {
			continue
		}

		// Check if property exists in documentation
		docProperty := documentation.Objects[name]
		if docProperty == nil {
			errs = append(errs, fmt.Errorf("%s: argument `%s` exists in schema but is missing from documentation",
				IdAndName(s), fullPath))
			continue
		}

		// Check for block type declarations
		if property.Nested != nil && len(property.Nested.Objects) > 0 {
			// Check if the field is marked as a block in documentation
			if !docProperty.Block {
				errs = append(errs, fmt.Errorf("%s: argument `%s` should be declared as a block (e.g., 'One or more `%s` block as defined below')",
					IdAndName(s), fullPath, name))
				continue
			}

			if docProperty.Nested == nil || len(docProperty.Nested.Objects) == 0 {
				// For some blocks sharing same sub-fields, they are defined in a shared and non-existed block section. e.g. azurerm_role_management_policy -> notification_target
				if docProperty.BlockTypeName != docProperty.Name {
					linkedDocProperty := blockDefinitions[docProperty.BlockTypeName]
					if linkedDocProperty != nil && linkedDocProperty.Nested != nil && len(linkedDocProperty.Nested.Objects) > 0 {
						errs = append(errs, s.checkMissingInDoc(resourceType, fullPath, property.Nested, linkedDocProperty.Nested, blockDefinitions)...)
						continue
					}
				}

				errs = append(errs, fmt.Errorf("%s: `%s` block is missing from documentation (e.g. A / An `%s` block supports the following:)",
					IdAndName(s), fullPath, name))
				continue
			}

			// Recursively check nested properties
			if docProperty.Nested != nil {
				errs = append(errs, s.checkMissingInDoc(resourceType, fullPath, property.Nested, docProperty.Nested, blockDefinitions)...)
			}
		}
	}

	return errs
}

// checkMissingInSchema checks if documented properties are missing from schema
func (s S006) checkMissingInSchema(resourceType, parentPath string, documentation *models.DocumentProperties, schema *models.SchemaProperties, blockDefinitions map[string]*models.DocumentProperty) []error {
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

		// Skip properties in skip config
		if isSkipProp(resourceType, fullPath) {
			continue
		}

		if len(docProperty.ParseErrors) > 0 {
			for _, parseErr := range docProperty.ParseErrors {
				if strings.Contains(parseErr, "misspell of name from") {
					errs = append(errs, fmt.Errorf("%s: argument `%s` has a misspelling: %s",
						IdAndName(s), fullPath, parseErr))
				} else {
					// Report other parse errors
					errs = append(errs, fmt.Errorf("%s: argument `%s` has parse error: %s",
						IdAndName(s), fullPath, parseErr))
				}
			}
			continue
		}

		// Check for deprecated properties in documentation content
		if strings.Contains(strings.ToLower(docProperty.Content), "deprecated") {
			continue
		}

		// Check if schema property exists
		schemaProperty := schema.Objects[name]
		if schemaProperty == nil {
			// Check for "not available for" pattern - specific property not available for certain block types
			if idx := strings.Index(strings.ToLower(docProperty.Content), "not available for"); idx > 0 {
				remaining := docProperty.Content[idx:]
				if codeValue := util.FirstCodeValue(remaining); codeValue != "" && strings.Contains(fullPath, codeValue) {
					continue
				}
			}

			errs = append(errs, fmt.Errorf("%s: argument `%s` is documented  but does not exist in schema",
				IdAndName(s), fullPath))
			continue
		}

		if docProperty.Nested != nil && len(docProperty.Nested.Objects) > 0 {
			if schemaProperty.Nested != nil {
				errs = append(errs, s.checkMissingInSchema(resourceType, fullPath, docProperty.Nested, schemaProperty.Nested, blockDefinitions)...)
			}
		}
	}

	return errs
}
