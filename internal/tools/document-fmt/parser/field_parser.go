// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package parser

import (
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/types"
)

// Type aliases for convenience
type (
	PositionType = types.PositionType
	RequiredType = types.RequiredType
)

// Re-export constants for backward compatibility
const (
	PosDefault = types.PosDefault
	PosExample = types.PosExample
	PosArgs    = types.PosArgs
	PosAttr    = types.PosAttr
	PosTimeout = types.PosTimeout
	PosImport  = types.PosImport
	PosOther   = types.PosOther

	RequiredDefault  = types.RequiredDefault
	RequiredOptional = types.RequiredOptional
	RequiredRequired = types.RequiredRequired
	RequiredComputed = types.RequiredComputed
)

// ParsedField represents a parsed field from markdown documentation
// This is a lightweight type used by the parser package to avoid circular dependencies
type ParsedField struct {
	Name           string
	Line           int
	Position       PositionType
	RequiredStatus RequiredType
	Required       bool
	Optional       bool
	Content        string
	DefaultValue   interface{}
	ForceNew       bool
	PossibleValues []string
	GuessEnums     []string
	EnumStart      int
	EnumEnd        int
	ParseErrors    []string
	Block          bool
	BlockTypeName  string
}

// ParsedProperty represents a complete property with nested structure
// Includes all fields from ParsedField plus nested properties
type ParsedProperty struct {
	ParsedField
	Type            string
	Description     string
	Computed        bool
	Deprecated      bool
	BlockHasSection bool
	Path            string
	Nested          *ParsedProperties
	SameNameAttr    *ParsedProperty
	NestedType      string
	AdditionalLines []string
	Count           int
}

// ParsedProperties represents a collection of parsed properties
type ParsedProperties struct {
	Names   []string
	Objects map[string]*ParsedProperty
}

// NewParsedProperties creates a new ParsedProperties instance
func NewParsedProperties() *ParsedProperties {
	return &ParsedProperties{
		Names:   make([]string, 0),
		Objects: make(map[string]*ParsedProperty),
	}
}

// AddProperty adds a property to the collection
func (props *ParsedProperties) AddProperty(p *ParsedProperty) {
	if props == nil {
		return
	}
	if p == nil || p.Name == "" {
		return
	}

	props.Names = append(props.Names, p.Name)
	props.Objects[p.Name] = p
}

// AddField adds a ParsedField to the collection by converting it to ParsedProperty
func (props *ParsedProperties) AddField(f *ParsedField) {
	if f == nil {
		return
	}
	p := &ParsedProperty{
		ParsedField: *f,
	}
	if f.Block {
		p.Nested = NewParsedProperties()
	}
	props.AddProperty(p)
}

// Field extraction patterns and logic ported from document-lint
var (
	fieldReg        = regexp.MustCompile("^[*-] *`(.*?)`" + ` +\- +(\(Required\)|\(Optional\))? ?(.*)`)
	codeReg         = regexp.MustCompile("`([^`]+)`")
	blockHeadReg    = regexp.MustCompile("^(an?|An?|The)[^`]+(`[a-zA-Z0-9_]+`[, and]*)+.*blocks?.*$")
	blockTypeReg    = regexp.MustCompile("`([a-zA-Z0-9_]+)`")
	DefaultsReg     = regexp.MustCompile("[.,?;](?: *[Tt]he)? *[Dd]efaults?[^`'\".]+(?:to|is) ('[^']+'|`[^`]+`|\"[^\"]+\")[ .,]?")
	ForceNewReg     = regexp.MustCompile(` ?Changing.*forces? a [^.]*(\.|$)`)
	partForceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]* created when [^.]*(\.|$)`)

	// Block property detection patterns (from document-lint)
	blockPropRegs = []*regexp.Regexp{
		regexp.MustCompile("(?:[Oo]ne|[Ee]ach|more(?: \\(.*\\))?|[Tt]he|as|of|[Aa]n?) ['\"`]([^ ]+)['\"`] (?:block|object)[^.]+(?:below|above)"),
	}
)

// getDefaultValue extracts default value from a field description line
func getDefaultValue(line string) string {
	if vals := DefaultsReg.FindStringSubmatch(line); len(vals) > 0 {
		if val := vals[1]; len(val) > 2 {
			return val[1 : len(val)-1]
		}
	}
	return ""
}

// isForceNew determines if a field forces new resource creation
func isForceNew(line string) bool {
	return ForceNewReg.MatchString(line) && !partForceNewReg.MatchString(line)
}

// FirstCodeValue extracts the first code value from a line
func FirstCodeValue(line string) string {
	matches := codeReg.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// IsBlockHead determines if a line defines a block
func IsBlockHead(line string) bool {
	return blockHeadReg.MatchString(line)
}

// ExtractBlockNames extracts block names from a block definition line
// Fixed to stop at "block" keyword (P0.3 improvement from document-lint)
func ExtractBlockNames(line string) []string {
	if !blockHeadReg.MatchString(line) {
		return nil
	}

	// Stop at "block" keyword to avoid extracting words after it (like "below")
	idx := strings.Index(line, "block")
	if idx <= 0 {
		return nil
	}

	// Extract code blocks only before "block" keyword
	matches := codeReg.FindAllString(line[:idx], -1)
	var names []string
	for _, match := range matches {
		name := strings.Trim(match, "`'")
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// guessBlockProperty determines if a field line describes a block type
// Based on document-lint's guessBlockProperty function
func guessBlockProperty(line string) bool {
	for _, reg := range blockPropRegs {
		if reg.MatchString(line) {
			return true
		}
	}
	return strings.Contains(line, "A block to")
}

// extractBlockTypeName extracts the block type name from a field line
// Based on document-lint's newFieldFromLine logic
func extractBlockTypeName(line string, fieldName string) string {
	blockTypeName := fieldName
	if match := blockTypeReg.FindAllStringSubmatchIndex(strings.ToLower(line), -1); len(match) > 0 {
		blockTypeName = line[match[0][2]:match[0][3]]
	}
	return blockTypeName
}

// ProcessBlockDefinition processes a block definition line and returns block metadata
func ProcessBlockDefinition(line string, position PositionType, lineNumber int) (blockNames []string, blockOf string) {
	blockNames = ExtractBlockNames(line)

	// Extract "of" or "within" relationships like document-lint does
	for _, sep := range []string{" of ", " within "} {
		if idx := strings.Index(line, sep); idx > 0 {
			blockOf = FirstCodeValue(line[idx:])
			break
		}
	}

	return blockNames, blockOf
}

// ExtractFieldFromLine parses a markdown field line into a ParsedField
func ExtractFieldFromLine(line string, position PositionType, lineNumber int) *ParsedField {
	field := &ParsedField{
		Content:  line,
		Line:     lineNumber,
		Position: position,
	}

	// Extract default value and force new flag
	if defaultVal := getDefaultValue(line); defaultVal != "" {
		field.DefaultValue = defaultVal
	}
	field.ForceNew = isForceNew(line)

	// Parse field using main regex
	res := fieldReg.FindStringSubmatch(line)
	if len(res) <= 1 || res[1] == "" {
		field.Name = FirstCodeValue(line) // try to use the first code as name
		if field.Name == "" {
			field.ParseErrors = append(field.ParseErrors, "no field name found")
			return field
		}
	} else {
		field.Name = res[1]
	}

	if field.Name == "" {
		log.Printf("field name is empty for line: %s", line)
		field.ParseErrors = append(field.ParseErrors, "field name is empty")
	}

	// Parse required/optional status
	if len(res) > 2 {
		switch {
		case strings.Contains(line, "(Required)"):
			field.RequiredStatus = RequiredRequired
			field.Required = true
		case strings.Contains(line, "(Optional)"):
			field.RequiredStatus = RequiredOptional
			field.Optional = true
		case strings.Contains(line, "Required"):
			field.RequiredStatus = RequiredRequired
			field.Required = true
		case strings.Contains(line, "Optional"):
			field.RequiredStatus = RequiredOptional
			field.Optional = true
		}
	}

	// Extract possible values/enums
	if len(res) > 3 {
		enums := extractPossibleValues(line, field)
		field.AddEnum(enums...)

		// Fallback: if no enums found but there are code blocks, guess them
		if len(field.PossibleValues) == 0 && strings.Index(res[3], "`") > 0 {
			guessValues := codeReg.FindAllString(res[3], -1)
			field.SetGuessEnums(guessValues)
		}
	}

	// Check if this field describes a block type
	if guessBlockProperty(line) {
		field.Block = true
		field.BlockTypeName = extractBlockTypeName(line, field.Name)
	}

	return field
}

// AddEnum adds enum values to PossibleValues while avoiding duplicates
func (f *ParsedField) AddEnum(values ...string) {
	existingMap := make(map[string]bool)
	for _, v := range f.PossibleValues {
		existingMap[v] = true
	}

	for _, value := range values {
		trimmed := strings.Trim(value, "`\"'")
		if trimmed != "" && !existingMap[trimmed] {
			f.PossibleValues = append(f.PossibleValues, trimmed)
			existingMap[trimmed] = true
		}
	}
}

// SetGuessEnums sets guess enum values after removing duplicates
func (f *ParsedField) SetGuessEnums(values []string) {
	seen := make(map[string]bool)
	var result []string
	for _, val := range values {
		val = strings.Trim(val, "`\"'")
		if val != "" && !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	f.GuessEnums = result
}

// extractPossibleValues extracts enum values from field description
func extractPossibleValues(line string, field *ParsedField) []string {
	possibleValueSep := func(line string) int {
		line = strings.ToLower(line)
		for _, sep := range []string{
			"possible value", "must be one of", "be one of", "allowed value", "valid value",
			"supported value", "valid option", "accepted value",
		} {
			if sepIdx := strings.Index(line, sep); sepIdx >= 0 {
				return sepIdx
			}
		}
		return -1
	}

	var enums []string

	// Find the "possible values" separator
	sepIdx := possibleValueSep(line)
	if sepIdx <= 0 {
		return enums
	}

	subStr := line[sepIdx:]
	field.EnumStart = sepIdx

	// Find the end of the possible values section (usually a period)
	pointEnd := strings.Index(subStr, ".")
	if pointEnd < 0 {
		pointEnd = len(subStr)
	}

	// Extract code blocks as enum values
	enumIndex := codeReg.FindAllStringIndex(subStr, -1)
	for _, val := range enumIndex {
		start, end := val[0], val[1]

		// Handle periods inside code blocks
		if pointEnd > start && pointEnd < end {
			if newPointEnd := strings.Index(subStr[end:], "."); newPointEnd >= 0 {
				pointEnd = end + newPointEnd
			} else {
				pointEnd = len(subStr)
			}
		}

		// Stop if we've reached the end of the possible values section
		if pointEnd < start {
			break
		}

		enums = append(enums, strings.Trim(subStr[start:end], "`'\""))
		field.EnumEnd = sepIdx + end
	}

	// Check for multiple possible value sections (skip if found)
	if possibleValueSep(line[sepIdx+1:]) >= 0 {
		field.ParseErrors = append(field.ParseErrors, "multiple possible value sections detected, skipping enum extraction")
	}

	return enums
}
