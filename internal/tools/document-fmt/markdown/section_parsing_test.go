// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package markdown

import (
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/parser"
)

func TestArgumentsSectionParseFields(t *testing.T) {
	content := []string{
		"The following arguments are supported:",
		"",
		"* `name` - (Required) The name of the resource.",
		"* `location` - (Optional) The location where the resource should be created.",
		"* `sku` - (Required) The SKU of the resource. Possible values are `Standard`, `Premium`, and `Basic`.",
		"* `force_new_field` - (Optional) A test field. Changing this forces a new resource to be created.",
		"",
		"---",
		"",
		"A `site_config` block supports the following:",
		"",
		"* `always_on` - (Optional) Should the Function App be loaded at all times? Defaults to `false`.",
		"* `http_logs_enabled` - (Optional) Should HTTP logs be enabled? Defaults to `false`.",
		"",
		"---",
		"",
		"An `identity` block supports the following:",
		"",
		"* `type` - (Required) The type of identity. Possible values are `SystemAssigned`, `UserAssigned`.",
	}

	// Create a mock ArgumentsSection
	section := &ArgumentsSection{}
	section.SetContent(content)

	props, err := section.ParseFields()
	if err != nil {
		t.Fatalf("ParseFields returned error: %v", err)
	}

	if props == nil {
		t.Fatalf("ParseFields returned nil properties")
	}

	// Check that we found the expected top-level fields and blocks
	expectedTopLevel := []string{"name", "location", "sku", "force_new_field", "site_config", "identity"}
	if len(props.Objects) != len(expectedTopLevel) {
		t.Errorf("Expected %d top-level properties, got %d", len(expectedTopLevel), len(props.Objects))
	}

	// Verify specific field properties
	nameField := props.Objects["name"]
	if nameField == nil {
		t.Fatalf("name field not found")
	}
	if nameField.RequiredStatus != parser.RequiredRequired {
		t.Errorf("Expected name field to be required, got %v", nameField.RequiredStatus)
	}

	skuField := props.Objects["sku"]
	if skuField == nil {
		t.Fatalf("sku field not found")
	}
	expectedEnums := []string{"Standard", "Premium", "Basic"}
	if len(skuField.PossibleValues) != len(expectedEnums) {
		t.Errorf("Expected %d enum values for sku, got %d", len(expectedEnums), len(skuField.PossibleValues))
	}

	forceNewField := props.Objects["force_new_field"]
	if forceNewField == nil {
		t.Fatalf("force_new_field not found")
	}
	if !forceNewField.ForceNew {
		t.Errorf("Expected force_new_field to have ForceNew=true")
	}

	// Test block parsing
	siteConfigBlock := props.Objects["site_config"]
	if siteConfigBlock == nil {
		t.Fatalf("site_config block not found")
	}
	if !siteConfigBlock.Block {
		t.Errorf("Expected site_config to be marked as block")
	}
	if siteConfigBlock.Nested == nil {
		t.Fatalf("site_config block has no nested properties")
	}
	if len(siteConfigBlock.Nested.Objects) != 2 {
		t.Errorf("Expected site_config block to have 2 nested properties, got %d", len(siteConfigBlock.Nested.Objects))
	}

	// Test nested field
	alwaysOnField := siteConfigBlock.Nested.Objects["always_on"]
	if alwaysOnField == nil {
		t.Fatalf("always_on field not found in site_config block")
	}
	if alwaysOnField.RequiredStatus != parser.RequiredOptional {
		t.Errorf("Expected always_on to be optional, got %v", alwaysOnField.RequiredStatus)
	}

	// Test identity block
	identityBlock := props.Objects["identity"]
	if identityBlock == nil {
		t.Fatalf("identity block not found")
	}
	if !identityBlock.Block {
		t.Errorf("Expected identity to be marked as block")
	}

	// Test nested field with enums in identity block
	typeField := identityBlock.Nested.Objects["type"]
	if typeField == nil {
		t.Fatalf("type field not found in identity block")
	}
	expectedIdentityEnums := []string{"SystemAssigned", "UserAssigned"}
	if len(typeField.PossibleValues) != len(expectedIdentityEnums) {
		t.Errorf("Expected %d enum values for identity.type, got %d", len(expectedIdentityEnums), len(typeField.PossibleValues))
	}
}

func TestAttributesSectionParseFields(t *testing.T) {
	content := []string{
		"In addition to the Arguments listed above, the following Attributes are exported:",
		"",
		"* `id` - The ID of the resource.",
		"* `fqdn` - The fully qualified domain name.",
		"* `status` - The status of the resource. Possible values are `Active`, `Inactive`.",
	}

	// Create a mock AttributesSection
	section := &AttributesSection{}
	section.SetContent(content)

	props, err := section.ParseFields()
	if err != nil {
		t.Fatalf("ParseFields returned error: %v", err)
	}

	if props == nil {
		t.Fatalf("ParseFields returned nil properties")
	}

	// Check that we found the expected fields
	if len(props.Objects) != 3 {
		t.Errorf("Expected 3 fields, got %d", len(props.Objects))
	}

	// Verify that position is set correctly for attributes
	for _, field := range props.Objects {
		if field.Position != parser.PosAttr {
			t.Errorf("Expected field position to be PosAttr, got %v", field.Position)
		}
	}

	statusField := props.Objects["status"]
	if statusField == nil {
		t.Fatalf("status field not found")
	}

	expectedEnums := []string{"Active", "Inactive"}
	if len(statusField.PossibleValues) != len(expectedEnums) {
		t.Errorf("Expected %d enum values for status, got %d", len(expectedEnums), len(statusField.PossibleValues))
	}
}
