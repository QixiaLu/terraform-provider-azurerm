// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package markdown

import (
	"regexp"
	"strings"
)

// StructuredParser provides document-lint style parsing capabilities
type StructuredParser struct {
	content string
	lines   []string
}

type ParsedField struct {
	Name           string
	Path           string
	Line           int
	Position       PositionType
	Required       RequiredType
	Default        string
	ForceNew       bool
	Content        string
	Description    string
	PossibleValues []string
	BlockType      string
	Nested         *ParsedProperties
}

type ParsedProperties struct {
	Fields map[string]*ParsedField
	Order  []string
}

type PositionType int
type RequiredType int

const (
	PosArgs PositionType = iota
	PosAttributes
	PosTimeouts
)

const (
	RequiredDefault RequiredType = iota
	RequiredOptional
	RequiredRequired
	RequiredComputed
)

// Port key regex patterns from document-lint (EXACT COPY)
var (
	fieldReg        = regexp.MustCompile("^[*-] *`(.*?)`" + ` +\- +(\(Required\)|\(Optional\))? ?(.*)`)
	blockHeadReg    = regexp.MustCompile("^(an?|An?|The)[^`]+(`[a-zA-Z0-9_]+`[, and]*)+.*blocks?.*$")
	defaultsReg     = regexp.MustCompile("[.,?;](?: *[Tt]he)? *[Dd]efaults?[^`'\".]+(?:to|is) ('[^']+'|`[^`]+`|\"[^\"]+\")[ .,]?")
	forceNewReg     = regexp.MustCompile(` ?Changing.*forces? a [^.]*(\.|$)`)
	partForceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]* created when [^.]*(\.|$)`)
	codeReg         = regexp.MustCompile("`([^`]+)`")

	// Block property detection regex from document-lint
	blockPropRegs = []*regexp.Regexp{
		regexp.MustCompile("(?:[Oo]ne|[Ee]ach|more(?: \\(.*\\))?|[Tt]he|as|of|[Aa]n?) ['\"`]([^ ]+)['\"`] (?:block|object)[^.]+(?:below|above)"),
	}
	blockTypeReg = blockPropRegs[0]
)

func NewStructuredParser(content string) *StructuredParser {
	return &StructuredParser{
		content: content,
		lines:   strings.Split(content, "\n"),
	}
}

func (p *StructuredParser) ParseFields() (*ParsedProperties, error) {
	properties := &ParsedProperties{
		Fields: make(map[string]*ParsedField),
		Order:  make([]string, 0),
	}

	currentPos := PosArgs // Start with Arguments by default

	for lineNum, line := range p.lines {
		// Determine section position
		if newPos := p.detectPosition(line); newPos != -1 {
			currentPos = newPos
			continue
		}

		// Skip block header lines (they don't represent fields themselves)
		if p.isBlockHeader(line) {
			continue
		}

		// Parse field lines
		if field := p.parseFieldLine(line, lineNum, currentPos); field != nil {
			properties.Fields[field.Name] = field
			properties.Order = append(properties.Order, field.Name)
		}
	}

	return properties, nil
}

func (p *StructuredParser) detectPosition(line string) PositionType {
	lower := strings.ToLower(line)
	if regexp.MustCompile(`## arguments? reference`).MatchString(lower) {
		return PosArgs
	}
	if regexp.MustCompile(`## attributes? reference`).MatchString(lower) {
		return PosAttributes
	}
	if regexp.MustCompile(`## timeouts?`).MatchString(lower) {
		return PosTimeouts
	}
	return -1
}

func (p *StructuredParser) isBlockHeader(line string) bool {
	// Detect block header lines like "An `identity` block supports the following:"
	return blockHeadReg.MatchString(line)
}

func (p *StructuredParser) parseFieldLine(line string, lineNum int, pos PositionType) *ParsedField {
	// EXACT PORT from document-lint extractFieldFromLine logic
	field := &ParsedField{
		Content:  line,
		Line:     lineNum,
		Position: pos,
	}

	// Extract default value and ForceNew flag
	field.Default = p.getDefaultValue(line)
	field.ForceNew = p.isForceNew(line)

	// Main field extraction using the exact regex from document-lint
	res := fieldReg.FindStringSubmatch(line)
	if len(res) <= 1 || res[1] == "" {
		// Try to use the first code value as name (document-lint fallback behavior)
		if codes := codeReg.FindAllString(line, -1); len(codes) > 0 {
			field.Name = strings.Trim(codes[0], "`'\"")
			// But mark this as a format error like document-lint does
			return nil // Skip fields that don't match the proper pattern
		}
		return nil
	}

	field.Name = res[1]
	if field.Name == "" {
		return nil
	}

	// Extract required/optional status - EXACT LOGIC from document-lint
	if len(res) > 2 {
		switch {
		case strings.Contains(line, "(Required)"):
			field.Required = RequiredRequired
		case strings.Contains(line, "(Optional)"):
			field.Required = RequiredOptional
		case strings.Contains(line, "Required"):
			field.Required = RequiredRequired
		case strings.Contains(line, "Optional"):
			field.Required = RequiredOptional
		}
	}

	// Extract possible values using the complex logic from document-lint
	if len(res) > 3 {
		field.PossibleValues = p.extractPossibleValues(line)

		// If no enums found but there are code blocks, use them as guess values (document-lint behavior)
		if len(field.PossibleValues) == 0 && strings.Index(res[3], "`") > 0 {
			guessValues := codeReg.FindAllString(res[3], -1)
			for i, val := range guessValues {
				guessValues[i] = strings.Trim(val, "`'\"")
			}
			// Store as guess values (document-lint has this concept)
			field.PossibleValues = guessValues
		}
	}

	// Detect if this is a block using document-lint logic
	if p.guessBlockProperty(line) {
		// EXACT COPY from document-lint newFieldFromLine
		field.BlockType = field.Name
		if match := blockTypeReg.FindAllStringSubmatchIndex(strings.ToLower(line), -1); len(match) > 0 {
			field.BlockType = line[match[0][2]:match[0][3]]
		}
	}

	return field
}

func (p *StructuredParser) getDefaultValue(line string) string {
	// EXACT COPY from document-lint
	if vals := defaultsReg.FindStringSubmatch(line); len(vals) > 0 {
		if val := vals[1]; len(val) > 2 {
			return val[1 : len(val)-1] // trim leading/tailing character
		}
	}
	return ""
}

func (p *StructuredParser) isForceNew(line string) bool {
	// EXACT COPY from document-lint
	if forceNewReg.MatchString(line) && !partForceNewReg.MatchString(line) {
		return true
	}
	return false
}

func (p *StructuredParser) extractPossibleValues(line string) []string {
	// EXACT COPY from document-lint extractFieldFromLine logic
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
	if sepIdx := possibleValueSep(line); sepIdx > 0 {
		subStr := line[sepIdx:]
		// end with dot may not work in values like `7.2` ....
		// should be . not in ` mark
		// Possible values are `a`, `b`, `a.b` and `def`.
		pointEnd := strings.Index(subStr, ".")
		if pointEnd < 0 {
			pointEnd = len(subStr)
		}
		enumIndex := codeReg.FindAllStringIndex(subStr, -1)
		for idx, val := range enumIndex {
			_ = idx
			start, end := val[0], val[1]
			if pointEnd > start && pointEnd < end {
				// point inside the code block
				if pointEnd = strings.Index(subStr[end:], "."); pointEnd < 0 {
					pointEnd = len(subStr)
				} else {
					pointEnd += end
				}
			}
			// search end to a dot
			if pointEnd < start {
				break
			}
			enums = append(enums, strings.Trim(subStr[start:end], "`'\""))
		}
		// breaks if there are more than 1 possible value
		if sepIdx = possibleValueSep(line[sepIdx+1:]); sepIdx >= 0 {
			// Skip if multiple possible value patterns found (document-lint behavior)
			return nil
		}
	}

	return enums
}

func (p *StructuredParser) guessBlockProperty(line string) bool {
	for _, reg := range blockPropRegs {
		if reg.MatchString(line) {
			return true
		}
	}

	return strings.Contains(line, "A block to")
}

func (p *StructuredParser) isBlockAsDefinedBelow(line string) bool {
	// Check for "An X block as defined below" pattern specifically
	return strings.Contains(line, "block as defined below")
}
