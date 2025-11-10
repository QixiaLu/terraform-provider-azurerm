// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/markdown"
)

func main() {
	// Example usage of the new structured parser
	sampleDoc := `# azurerm_storage_account

## Arguments Reference

* ` + "`name`" + ` - (Required) The name of the storage account. Changing this forces a new resource to be created.

* ` + "`location`" + ` - (Required) The Azure Region where the Storage Account should exist.

* ` + "`account_tier`" + ` - (Required) Defines the Tier to use for this storage account. Valid options are ` + "`Standard`" + ` and ` + "`Premium`" + `.

* ` + "`account_replication_type`" + ` - (Required) Defines the type of replication to use. Valid options are ` + "`LRS`" + `, ` + "`GRS`" + `, ` + "`RAGRS`" + `, ` + "`ZRS`" + `, ` + "`GZRS`" + ` and ` + "`RAGZRS`" + `.

* ` + "`tags`" + ` - (Optional) A mapping of tags to assign to the resource.

## Attributes Reference

* ` + "`id`" + ` - The ID of the Storage Account.

* ` + "`primary_endpoint`" + ` - The endpoint URL for blob storage in the primary location.

* ` + "`secondary_endpoint`" + ` - The endpoint URL for blob storage in the secondary location.
`

	fmt.Println("=== Structured Markdown Parser Demo ===")
	fmt.Println()

	parser := markdown.NewStructuredParser(sampleDoc)
	properties, err := parser.ParseFields()
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	fmt.Printf("Parsed %d fields:\n", len(properties.Fields))
	fmt.Println()

	// Demonstrate parsed fields grouped by position
	argFields := make([]*markdown.ParsedField, 0)
	attrFields := make([]*markdown.ParsedField, 0)

	for _, name := range properties.Order {
		field := properties.Fields[name]
		switch field.Position {
		case markdown.PosArgs:
			argFields = append(argFields, field)
		case markdown.PosAttributes:
			attrFields = append(attrFields, field)
		}
	}

	fmt.Println("📝 Arguments:")
	for _, field := range argFields {
		required := "Optional"
		if field.Required == markdown.RequiredRequired {
			required = "Required"
		}
		
		fmt.Printf("  • %s (%s)", field.Name, required)
		if field.ForceNew {
			fmt.Printf(" [ForceNew]")
		}
		if field.Default != "" {
			fmt.Printf(" [Default: %s]", field.Default)
		}
		if len(field.PossibleValues) > 0 {
			fmt.Printf(" [Values: %v]", field.PossibleValues)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("📊 Attributes:")
	for _, field := range attrFields {
		fmt.Printf("  • %s", field.Name)
		if field.Description != "" {
			fmt.Printf(" - %s", field.Description)
		}
		fmt.Println()
	}

	fmt.Println()
	fmt.Println("✅ Structured parsing completed successfully!")
	fmt.Println()
	
	// Show some analysis
	requiredCount := 0
	optionalCount := 0
	forceNewCount := 0
	enumCount := 0
	
	for _, field := range properties.Fields {
		switch field.Required {
		case markdown.RequiredRequired:
			requiredCount++
		case markdown.RequiredOptional:
			optionalCount++
		}
		
		if field.ForceNew {
			forceNewCount++
		}
		
		if len(field.PossibleValues) > 0 {
			enumCount++
		}
	}
	
	fmt.Println("📈 Analysis:")
	fmt.Printf("  • Required fields: %d\n", requiredCount)
	fmt.Printf("  • Optional fields: %d\n", optionalCount)
	fmt.Printf("  • ForceNew fields: %d\n", forceNewCount)
	fmt.Printf("  • Fields with enums: %d\n", enumCount)
}