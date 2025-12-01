// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package models

// DocumentProperty represents a property parsed from documentation (markdown)
type DocumentProperty struct {
	Name     string
	Required bool
	Optional bool
	ForceNew bool // Whether documentation mentions "forces new"

	PossibleValues []string
	DefaultValue   string

	// Block related attributes
	Block         bool
	BlockTypeName string // Block type name (may differ from field name)
	Nested        *DocumentProperties

	// Documentation metadata
	Line            int      // Source line number in documentation
	Content         string   // Original markdown line content
	AdditionalLines []string // Tracks any lines from docs beyond initial property

	// Parsing metadata
	EnumStart   int      // Start position of enum values in content
	EnumEnd     int      // End position of enum values in content
	ParseErrors []string // Errors encountered during parsing
	GuessEnums  []string // Guessed enum values from code blocks
	Skip        bool     // Whether this field should be skipped in validation (e.g., multiple possible value sections)
	Count       int      // Property count, for doc parsing to detect duplicate entries
	Path        string   // xpath-like path (a.b.c)
}

// DocumentProperties represents a collection of document properties
type DocumentProperties struct {
	Names            []string                     // Tracks property ordering in documentation
	Objects          map[string]*DocumentProperty // Property definitions
	BlockDefinitions map[string]*DocumentProperty // Block definitions ("A `name` block supports:")
}

func NewDocumentProperties() *DocumentProperties {
	return &DocumentProperties{
		Names:            make([]string, 0),
		Objects:          make(map[string]*DocumentProperty),
		BlockDefinitions: make(map[string]*DocumentProperty),
	}
}

func (p *DocumentProperty) HasParseErrors() bool {
	return len(p.ParseErrors) > 0
}

// ShouldSkip returns true if the property should be skipped in validation
func (p *DocumentProperty) ShouldSkip() bool {
	return p.Skip
}
