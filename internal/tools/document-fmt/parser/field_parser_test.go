// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"testing"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/types"
)

func TestGuessBlockProperty(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		{
			name:     "identity block pattern",
			line:     "* `identity` - (Optional) An `identity` block as defined below.",
			expected: true,
		},
		{
			name:     "the block pattern",
			line:     "* `network_profile` - (Optional) The `network_profile` block as defined below.",
			expected: true,
		},
		{
			name:     "one block pattern",
			line:     "* `storage` - (Optional) One `storage` block as defined below.",
			expected: true,
		},
		{
			name:     "A block to pattern",
			line:     "* `settings` - (Optional) A block to configure settings.",
			expected: true,
		},
		{
			name:     "regular field",
			line:     "* `name` - (Required) The name of the resource.",
			expected: false,
		},
		{
			name:     "field with code block but not block type",
			line:     "* `enabled` - (Optional) Is this `enabled`? Defaults to `true`.",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := guessBlockProperty(tt.line)
			if result != tt.expected {
				t.Errorf("guessBlockProperty(%q) = %v, expected %v", tt.line, result, tt.expected)
			}
		})
	}
}

func TestExtractBlockNames(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "single block name",
			line:     "An `identity` block supports the following:",
			expected: []string{"identity"},
		},
		{
			name:     "multiple block names",
			line:     "An `identity` or `principal` block supports the following:",
			expected: []string{"identity", "principal"},
		},
		{
			name:     "stops at block keyword",
			line:     "An `identity` block as defined `below`:",
			expected: []string{"identity"},
		},
		{
			name:     "The block pattern",
			line:     "The `network_profile` block supports:",
			expected: []string{"network_profile"},
		},
		{
			name:     "not a block definition",
			line:     "* `name` - (Required) The name.",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBlockNames(tt.line)
			if len(result) != len(tt.expected) {
				t.Errorf("ExtractBlockNames(%q) returned %v, expected %v", tt.line, result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("ExtractBlockNames(%q)[%d] = %q, expected %q", tt.line, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestExtractFieldFromLine_BlockDetection(t *testing.T) {
	tests := []struct {
		name              string
		line              string
		shouldBeBlock     bool
		expectedBlockType string
	}{
		{
			name:              "identity block field",
			line:              "* `identity` - (Optional) An `identity` block as defined below.",
			shouldBeBlock:     true,
			expectedBlockType: "identity",
		},
		{
			name:              "network_profile block field",
			line:              "* `network_profile` - (Optional) The `network_profile` block as defined below.",
			shouldBeBlock:     true,
			expectedBlockType: "network_profile",
		},
		{
			name:          "regular string field",
			line:          "* `name` - (Required) The name of the resource.",
			shouldBeBlock: false,
		},
		{
			name:          "field with possible values",
			line:          "* `type` - (Required) The type. Possible values are `SystemAssigned` and `UserAssigned`.",
			shouldBeBlock: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := ExtractFieldFromLine(tt.line, types.PosArgs, 1)
			if field == nil {
				t.Fatal("ExtractFieldFromLine returned nil")
			}

			if field.Block != tt.shouldBeBlock {
				t.Errorf("field.Block = %v, expected %v", field.Block, tt.shouldBeBlock)
			}

			if tt.shouldBeBlock && field.BlockTypeName != tt.expectedBlockType {
				t.Errorf("field.BlockTypeName = %q, expected %q", field.BlockTypeName, tt.expectedBlockType)
			}
		})
	}
}
