// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package data

import (
	"log"
	"regexp"
	"strings"
)

// Field extraction patterns and logic ported
var (
	fieldReg        = regexp.MustCompile("^[*-] *`(.*?)`" + ` +\- +(\(Required\)|\(Optional\))? ?(.*)`)
	codeReg         = regexp.MustCompile("`([^`]+)`")
	blockHeadReg    = regexp.MustCompile("^(an?|An?|The)[^`]+(`[a-zA-Z0-9_]+`[, and]*)+.*blocks?.*$")
	blockTypeReg    = regexp.MustCompile("`([a-zA-Z0-9_]+)`")
	DefaultsReg     = regexp.MustCompile("[.,?;](?: *[Tt]he)? *[Dd]efaults?[^`'\".]+(?:to|is) ('[^']+'|`[^`]+`|\"[^\"]+\")[ .,]?")
	ForceNewReg     = regexp.MustCompile(` ?Changing.*forces? a [^.]*(\.|$)`)
	partForceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]* created when [^.]*(\.|$)`)

	// Block property detection patterns
	blockPropRegs = []*regexp.Regexp{
		regexp.MustCompile("(?:[Oo]ne|[Ee]ach|more(?: \\(.*\\))?|[Tt]he|as|of|[Aa]n?) ['\"`]([^ ]+)['\"`] (?:block|object)[^.]+(?:below|above)"),
	}
)

// ExtractFieldFromLine parses a markdown field line into a ParsedField
func ExtractFieldFromLine(line string, position PositionType, lineNumber int) *Property {
	field := &Property{
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
			field.Required = true
		case strings.Contains(line, "(Optional)"):
			field.Optional = true
		case strings.Contains(line, "Required"):
			field.Required = true
		case strings.Contains(line, "Optional"):
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

// isBlockHead determines if this line is the start of a block
func isBlockHead(line string) bool {
	return blockHeadReg.MatchString(line)
}

// processBlockDefinition processes a block definition line and returns block metadata
func processBlockDefinition(line string, position PositionType, lineNumber int) (blockNames []string, blockOf string) {
	blockNames = extractBlockNames(line)

	for _, sep := range []string{" of ", " within "} {
		if idx := strings.Index(line, sep); idx > 0 {
			blockOf = FirstCodeValue(line[idx:])
			break
		}
	}

	return blockNames, blockOf
}

func extractBlockNames(line string) []string {
	if !blockHeadReg.MatchString(line) {
		return nil
	}

	idx := strings.Index(line, "block")
	if idx <= 0 {
		return nil
	}

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

// FirstCodeValue extracts the first code value from a line
func FirstCodeValue(line string) string {
	matches := codeReg.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractPossibleValues extracts enum values from field description
func extractPossibleValues(line string, field *Property) []string {
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


// guessBlockProperty determines if a field line describes a block type
func guessBlockProperty(line string) bool {
	for _, reg := range blockPropRegs {
		if reg.MatchString(line) {
			return true
		}
	}
	return strings.Contains(line, "A block to")
}

// extractBlockTypeName extracts the block type name from a field line
func extractBlockTypeName(line string, fieldName string) string {
	blockTypeName := fieldName
	if match := blockTypeReg.FindAllStringSubmatchIndex(strings.ToLower(line), -1); len(match) > 0 {
		blockTypeName = line[match[0][2]:match[0][3]]
	}
	return blockTypeName
}
