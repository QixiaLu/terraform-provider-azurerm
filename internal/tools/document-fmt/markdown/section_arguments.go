// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package markdown

import (
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/parser"
)

type ArgumentsSection struct {
	heading      Heading
	content      []string
	parsedFields *parser.ParsedProperties // cached parsed fields
}

var _ SectionWithTemplate = &ArgumentsSection{}

func (s *ArgumentsSection) Match(line string) bool {
	return regexp.MustCompile(`#+(\s)*argument.*`).MatchString(strings.ToLower(line))
}

func (s *ArgumentsSection) SetHeading(line string) {
	s.heading = NewHeading(line)
}

func (s *ArgumentsSection) GetHeading() Heading {
	return s.heading
}

func (s *ArgumentsSection) SetContent(content []string) {
	s.content = content
	// Clear cached parsed fields when content changes
	s.parsedFields = nil
}

func (s *ArgumentsSection) GetContent() []string {
	return s.content
}

// ParseFields extracts structured field information from section content
func (s *ArgumentsSection) ParseFields() (*parser.ParsedProperties, error) {
	// Return cached result if available
	if s.parsedFields != nil {
		return s.parsedFields, nil
	}

	properties := parser.NewParsedProperties()
	var currentBlock *parser.ParsedProperty
	var inBlock bool

	for lineNum, line := range s.content {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "<!--") {
			continue
		}

		// Check if this is a block definition line
		if parser.IsBlockHead(line) {
			// Finish previous block if any
			if inBlock && currentBlock != nil {
				properties.AddProperty(currentBlock)
			}

			// Start new block
			blockNames, blockOf := parser.ProcessBlockDefinition(line, parser.PosArgs, lineNum)
			if len(blockNames) > 0 {
				currentBlock = &parser.ParsedProperty{
					Name:          blockNames[0],
					Block:         true,
					BlockTypeName: blockNames[0],
					Position:      parser.PosArgs,
					Line:          lineNum,
					Content:       line,
					Nested:        parser.NewParsedProperties(),
				}

				// Handle "block of" relationships
				if blockOf != "" {
					currentBlock.Path = blockOf + "." + currentBlock.Name
				}

				inBlock = true
			}
			continue
		}

		// Check for block section separator
		if line == "---" {
			if inBlock && currentBlock != nil {
				properties.AddProperty(currentBlock)
				currentBlock = nil
			}
			inBlock = false
			continue
		}

		// Check if this is a field line (starts with * or -)
		if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "-") {
			// Extract field using parser logic
			field := parser.ExtractFieldFromLine(line, parser.PosArgs, lineNum)
			if field != nil && field.Name != "" {
				if inBlock && currentBlock != nil {
					// Add to current block
					currentBlock.Nested.AddProperty(field)
				} else {
					// Add as top-level property
					properties.AddProperty(field)
				}
			}
		}
	}

	// Add any remaining block
	if inBlock && currentBlock != nil {
		properties.AddProperty(currentBlock)
	}

	// Cache the result
	s.parsedFields = properties
	return properties, nil
}

func (s *ArgumentsSection) Template() string {
	// TODO implement me
	panic("implement me")
}
