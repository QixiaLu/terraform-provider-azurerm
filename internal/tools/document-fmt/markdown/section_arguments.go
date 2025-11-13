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

	// TODO: lineNum is wrong here, it counts from the arg body
	for lineNum, line := range s.content {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmedLine == "" || strings.HasPrefix(trimmedLine, "<!--") {
			continue
		}

		// Skip notes
		// TODO: probably add it to the previous feilds' contents?
		if strings.HasPrefix(trimmedLine, "->") || strings.HasPrefix(trimmedLine, "~>") {
			continue
		}

		// Check if this is a block definition line
		if parser.IsBlockHead(trimmedLine) {
			// Finish previous block if any
			if inBlock && currentBlock != nil {
				properties.AddProperty(currentBlock)
			}

			// Start new block
			blockNames, blockOf := parser.ProcessBlockDefinition(trimmedLine, parser.PosArgs, lineNum)
			if len(blockNames) > 0 {
				currentBlock = &parser.ParsedProperty{
					ParsedField: parser.ParsedField{
						Name:          blockNames[0],
						Block:         true,
						BlockTypeName: blockNames[0],
						Position:      parser.PosArgs,
						Line:          lineNum,
						Content:       line,
					},
					Nested: parser.NewParsedProperties(),
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
		if trimmedLine == "---" {
			if inBlock && currentBlock != nil {
				properties.AddProperty(currentBlock)
				currentBlock = nil
			}
			inBlock = false
			continue
		}

		// Check if this is a field line (starts with * or -)
		if strings.HasPrefix(trimmedLine, "*") || strings.HasPrefix(trimmedLine, "-") {
			// Extract field using parser logic
			field := parser.ExtractFieldFromLine(trimmedLine, parser.PosArgs, lineNum)
			if field != nil && field.Name != "" {
				if inBlock && currentBlock != nil {
					// Add to current block
					currentBlock.Nested.AddField(field)
				} else {
					// Add as top-level property
					properties.AddField(field)
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
