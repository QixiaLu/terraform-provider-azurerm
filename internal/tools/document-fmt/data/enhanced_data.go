// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package data

import (
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/markdown"
	"github.com/spf13/afero"
)

// EnhancedTerraformNodeData extends TerraformNodeData with structured parsing capabilities
type EnhancedTerraformNodeData struct {
	*TerraformNodeData
	
	// New structured parsing results
	ParsedDocument *markdown.ParsedProperties
	StructuredData *StructuredDocumentData
}

type StructuredDocumentData struct {
	Arguments  *markdown.ParsedProperties
	Attributes *markdown.ParsedProperties
	Timeouts   *markdown.ParsedProperties
	Blocks     map[string]*markdown.ParsedProperties
}

// ParseDocumentStructure adds structured parsing capability to existing TerraformNodeData
func (t *TerraformNodeData) ParseDocumentStructure() (*EnhancedTerraformNodeData, error) {
	if t.Document == nil {
		return &EnhancedTerraformNodeData{TerraformNodeData: t}, nil
	}

	// Get the raw document content
	content := t.Document.GetContent()
	
	// Create structured parser
	parser := markdown.NewStructuredParser(content)
	
	// Parse all fields with position information
	parsedFields, err := parser.ParseFields()
	if err != nil {
		return nil, err
	}

	// Separate fields by position
	structuredData := &StructuredDocumentData{
		Arguments:  &markdown.ParsedProperties{Fields: make(map[string]*markdown.ParsedField), Order: make([]string, 0)},
		Attributes: &markdown.ParsedProperties{Fields: make(map[string]*markdown.ParsedField), Order: make([]string, 0)},
		Timeouts:   &markdown.ParsedProperties{Fields: make(map[string]*markdown.ParsedField), Order: make([]string, 0)},
		Blocks:     make(map[string]*markdown.ParsedProperties),
	}

	for _, name := range parsedFields.Order {
		field := parsedFields.Fields[name]
		
		switch field.Position {
		case markdown.PosArgs:
			structuredData.Arguments.Fields[name] = field
			structuredData.Arguments.Order = append(structuredData.Arguments.Order, name)
		case markdown.PosAttributes:
			structuredData.Attributes.Fields[name] = field
			structuredData.Attributes.Order = append(structuredData.Attributes.Order, name)
		case markdown.PosTimeouts:
			structuredData.Timeouts.Fields[name] = field
			structuredData.Timeouts.Order = append(structuredData.Timeouts.Order, name)
		}

		// Handle blocks
		if field.BlockType != "" {
			if structuredData.Blocks[field.BlockType] == nil {
				structuredData.Blocks[field.BlockType] = &markdown.ParsedProperties{
					Fields: make(map[string]*markdown.ParsedField),
					Order:  make([]string, 0),
				}
			}
			if field.Nested != nil {
				// Add nested fields to the block
				for nestedName, nestedField := range field.Nested.Fields {
					structuredData.Blocks[field.BlockType].Fields[nestedName] = nestedField
					structuredData.Blocks[field.BlockType].Order = append(structuredData.Blocks[field.BlockType].Order, nestedName)
				}
			}
		}
	}

	enhanced := &EnhancedTerraformNodeData{
		TerraformNodeData: t,
		ParsedDocument:    parsedFields,
		StructuredData:    structuredData,
	}

	// Update existing Properties for backward compatibility
	enhanced.DocumentArguments = toProperties(structuredData.Arguments)
	enhanced.DocumentAttributes = toProperties(structuredData.Attributes)

	return enhanced, nil
}

// GetAllEnhancedTerraformNodeData creates enhanced data for all resources
func GetAllEnhancedTerraformNodeData(fs afero.Fs, providerDirectory, service, resource string) ([]*EnhancedTerraformNodeData, error) {
	// Get regular terraform node data
	regularData := GetAllTerraformNodeData(fs, providerDirectory, service, resource)
	
	enhanced := make([]*EnhancedTerraformNodeData, 0, len(regularData))
	
	for _, data := range regularData {
		enhancedData, err := data.ParseDocumentStructure()
		if err != nil {
			// Log error but continue processing other resources
			data.Errors = append(data.Errors, err)
			enhancedData = &EnhancedTerraformNodeData{TerraformNodeData: data}
		}
		enhanced = append(enhanced, enhancedData)
	}
	
	return enhanced, nil
}

// Validation helpers using structured data
func (e *EnhancedTerraformNodeData) ValidateFieldMetadata() []error {
	var errors []error
	
	if e.ParsedDocument == nil {
		return errors
	}

	// Example validation: check for missing required fields in schema
	if e.SchemaProperties != nil {
		for schemaName, schemaProp := range e.SchemaProperties.Objects {
			if schemaProp.Required {
				// Check if documented in arguments
				if argField := e.StructuredData.Arguments.Fields[schemaName]; argField == nil {
					errors = append(errors, NewValidationError(
						e.Name, 
						"missing_required_field", 
						"Required field '%s' is not documented in Arguments section", 
						schemaName,
					))
				} else if argField.Required != markdown.RequiredRequired {
					errors = append(errors, NewValidationError(
						e.Name,
						"incorrect_required_flag",
						"Field '%s' should be marked as (Required) but is marked as %v",
						schemaName, argField.Required,
					))
				}
			}
		}
	}

	return errors
}

func (e *EnhancedTerraformNodeData) ValidateEnumValues() []error {
	var errors []error
	
	if e.ParsedDocument == nil {
		return errors
	}

	// Example: validate that documented enum values match schema
	for _, field := range e.ParsedDocument.Fields {
		if len(field.PossibleValues) > 0 {
			// Here you could cross-reference with schema to validate enum values
			// This is just a placeholder for the validation logic
		}
	}

	return errors
}

// Helper function to create validation errors
func NewValidationError(resource, errorType, format string, args ...interface{}) error {
	// Implementation would depend on existing error structures
	// This is a placeholder
	return nil
}

// toProperties converts ParsedProperties to Properties for backward compatibility
func toProperties(parsed *markdown.ParsedProperties) *Properties {
	if parsed == nil {
		return NewProperties()
	}

	props := NewProperties()
	
	for _, name := range parsed.Order {
		field := parsed.Fields[name]
		prop := &Property{
			Name:           field.Name,
			Description:    field.Description,
			Required:       field.Required == markdown.RequiredRequired,
			Optional:       field.Required == markdown.RequiredOptional,
			Computed:       field.Required == markdown.RequiredComputed,
			ForceNew:       field.ForceNew,
			PossibleValues: field.PossibleValues,
			Block:          field.BlockType != "",
		}
		
		if field.Default != "" {
			prop.DefaultValue = field.Default
		}
		
		props.Names = append(props.Names, name)
		props.Objects[name] = prop
	}
	
	return props
}