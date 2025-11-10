// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package markdown

import (
	"testing"
)

func TestStructuredParser_ParseFields(t *testing.T) {
	testMarkdown := `# azurerm_example_resource

## Arguments Reference

* ` + "`name`" + ` - (Required) The name of the resource. Changing this forces a new resource to be created.

* ` + "`location`" + ` - (Optional) The Azure Region where the Resource should exist. Defaults to ` + "`West Europe`" + `.

* ` + "`storage_type`" + ` - (Required) The storage type. Possible values are ` + "`Standard_LRS`" + `, ` + "`Premium_LRS`" + ` and ` + "`Standard_GRS`" + `.

* ` + "`identity`" + ` - (Optional) An ` + "`identity`" + ` block as defined below.

An ` + "`identity`" + ` block supports the following:

* ` + "`type`" + ` - (Required) The identity type. Possible values are ` + "`SystemAssigned`" + ` and ` + "`UserAssigned`" + `.

## Attributes Reference

* ` + "`id`" + ` - The ID of the Resource.

* ` + "`endpoint`" + ` - The endpoint URL of the resource.
`

	parser := NewStructuredParser(testMarkdown)
	properties, err := parser.ParseFields()

	if err != nil {
		t.Fatalf("Failed to parse fields: %v", err)
	}

	// Test basic field extraction
	if properties.Fields["name"] == nil {
		t.Error("Expected 'name' field to be parsed")
	}

	nameField := properties.Fields["name"]
	if nameField.Required != RequiredRequired {
		t.Error("Expected 'name' field to be required")
	}

	if !nameField.ForceNew {
		t.Error("Expected 'name' field to have ForceNew=true")
	}

	// Test default value extraction
	locationField := properties.Fields["location"]
	if locationField == nil {
		t.Error("Expected 'location' field to be parsed")
	}

	if locationField.Default != "West Europe" {
		t.Errorf("Expected default value 'West Europe', got '%s'", locationField.Default)
	}

	// Test possible values extraction
	storageField := properties.Fields["storage_type"]
	if storageField == nil {
		t.Error("Expected 'storage_type' field to be parsed")
	}

	expectedValues := []string{"Standard_LRS", "Premium_LRS", "Standard_GRS"}
	if len(storageField.PossibleValues) != len(expectedValues) {
		t.Errorf("Expected %d possible values, got %d", len(expectedValues), len(storageField.PossibleValues))
	}

	// Test block detection
	identityField := properties.Fields["identity"]
	if identityField == nil {
		t.Error("Expected 'identity' field to be parsed")
	}

	if identityField.BlockType == "" {
		t.Error("Expected 'identity' field to be detected as a block")
	}

	// Test position tracking
	idField := properties.Fields["id"]
	if idField == nil {
		t.Error("Expected 'id' field to be parsed")
	}

	if idField.Position != PosAttributes {
		t.Error("Expected 'id' field to be in Attributes position")
	}

	// Test field ordering
	if len(properties.Order) == 0 {
		t.Error("Expected field ordering to be maintained")
	}
}

func TestStructuredParser_Integration(t *testing.T) {
	testMarkdown := `## Arguments Reference

* ` + "`name`" + ` - (Required) The name of the resource.

* ` + "`tags`" + ` - (Optional) A mapping of tags to assign to the resource.
`

	parser := NewStructuredParser(testMarkdown)
	parsed, err := parser.ParseFields()
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Test basic parsing
	if len(parsed.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(parsed.Fields))
	}

	if parsed.Fields["name"] == nil || parsed.Fields["name"].Required != RequiredRequired {
		t.Error("Name field should be required")
	}

	if parsed.Fields["tags"] == nil || parsed.Fields["tags"].Required != RequiredOptional {
		t.Error("Tags field should be optional")
	}
}