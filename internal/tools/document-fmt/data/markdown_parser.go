// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package data

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/util"
)

type itemType int

const (
	itemDefault itemType = iota
	itemHeader
	itemMeteInfo
	itemExample
	itemField
	itemBlockHead
	itemNote
	itemSeparator
	itemPlainText
)

const (
	BlcokNotDefined        = "block is not defined in the documentation"
	IncorrectlyBlockMarked = "The document incorrectly implies this field is a block"
)

type markItem struct {
	fromLine int
	toLine   int
	lines    []string
	itemType itemType
	field    *Property
}

func (m *markItem) content() string {
	return strings.Join(m.lines, "\n")
}

func (m *markItem) addLine(num int, line string) {
	m.lines = append(m.lines, line)
	m.toLine = num
}

func newMarkItem(fromLine int, content string, typ itemType) *markItem {
	return &markItem{
		fromLine: fromLine,
		lines:    []string{content},
		itemType: typ,
	}
}

type markBlock struct {
	Names    []string
	Of       string
	Name     string
	HeadLine int
	Fields   []*Property
	asProp   *Properties
}

func (b *markBlock) asProperties() *Properties {
	if b.asProp == nil {
		res := NewProperties()
		for _, f := range b.Fields {
			if _, ok := res.Objects[f.Name]; ok {
				if f.ParseErrors == nil {
					f.ParseErrors = []string{}
				}
				f.ParseErrors = append(f.ParseErrors, fmt.Sprintf("duplicate field `%s` in block %s", f.Name, b.Name))
			}
			res.Objects[f.Name] = f
			res.Names = append(res.Names, f.Name)
		}
		b.asProp = res
	}
	return b.asProp
}

func (b *markBlock) addField(f *Property) {
	b.Fields = append(b.Fields, f)
}

type mark struct {
	items  []*markItem
	blocks []markBlock
	fields map[string]*Property
}

func (m *mark) lastItem() *markItem {
	if len(m.items) > 0 {
		return m.items[len(m.items)-1]
	}
	return nil
}

func (m *mark) addItem(item *markItem) {
	m.items = append(m.items, item)
}

func (m *mark) addItemWith(num int, line string, typ itemType) {
	m.addItem(newMarkItem(num, line, typ))
}

func (m *mark) addLineOrItem(num int, line string, typ itemType) {
	last := m.lastItem()
	if last != nil && last.itemType == typ {
		last.addLine(num, line)
	} else {
		m.addItem(newMarkItem(num, line, typ))
	}
}

func (m *mark) addBlock(b markBlock) {
	m.blocks = append(m.blocks, b)
}

// newMarkFromString performs Phase 1: Parse markdown into structured items
func newMarkFromString(content string) *mark {
	lines := strings.Split(content, "\n")
	result := &mark{
		fields: map[string]*Property{},
	}

	for idx, line := range lines {
		switch {
		case strings.HasPrefix(line, "##"):
			result.addItem(newMarkItem(idx, line, itemHeader))
		case strings.HasPrefix(line, "*"):
			result.addItem(newMarkItem(idx, line, itemField))
		case strings.HasPrefix(line, "---"):
			if idx == 0 {
				result.addItem(newMarkItem(idx, line, itemMeteInfo)) // TODO: remove? this seems to be the example
			} else {
				last := result.lastItem()
				if last != nil && last.itemType == itemMeteInfo {
					last.addLine(idx, line)
				} else {
					result.addItem(newMarkItem(idx, line, itemSeparator))
				}
			}
		case strings.HasPrefix(line, "```"):
			result.addLineOrItem(idx, line, itemExample)
		case strings.HasPrefix(line, "->"), strings.HasPrefix(line, "~>"), strings.HasPrefix(line, "!>"):
			result.addItem(newMarkItem(idx, line, itemNote))
		case isBlockHead(line):
			result.addItem(newMarkItem(idx, line, itemBlockHead))
		default:
			// plain text
			last := result.lastItem()
			if last == nil {
				result.addItem(newMarkItem(idx, line, itemPlainText))
				continue
			}
			switch last.itemType {
			case itemField, itemMeteInfo, itemPlainText:
				last.addLine(idx, line)
			default:
				if strings.TrimSpace(line) == "" {
					last.addLine(idx, line)
				} else {
					result.addItem(newMarkItem(idx, line, itemPlainText))
				}
			}
		}
	}

	result.buildField()
	result.buildStruct()
	return result
}

// buildField performs Phase 2: Build field and block structures from items
func (m *mark) buildField() {
	var inBlock bool
	var block markBlock

	for _, item := range m.items {
		content := item.content()
		switch item.itemType {
		case itemField:
			f := newFieldFromLine(content)
			f.Line = item.fromLine
			item.field = f
			if inBlock {
				block.addField(f)
			} else {
				if arg, ok := m.fields[f.Name]; ok {
					if arg.ParseErrors == nil {
						arg.ParseErrors = []string{}
					}
					arg.ParseErrors = append(arg.ParseErrors, "duplicate fields declared")
				} else {
					m.fields[f.Name] = f
				}
			}
		case itemBlockHead:
			if inBlock {
				m.addBlock(block)
			}
			names := extractBlockNames(item.lines[0])
			// of/within block
			var of string
			for _, sep := range []string{" of ", " within "} {
				if idx := strings.Index(content, sep); idx > 0 {
					of = util.FirstCodeValue(content[idx:])
				}
			}

			block = markBlock{
				Names:    names,
				Name:     names[0],
				Of:       of,
				HeadLine: item.fromLine,
			}
			inBlock = true
		case itemSeparator:
			if inBlock {
				m.addBlock(block)
			}
			inBlock = false
		}
	}

	if inBlock {
		m.addBlock(block)
	}
}

// buildStruct performs Phase 3: Link block-type fields to their definitions
func (m *mark) buildStruct() {
	fillField := func(f *Property, parent string) {
		if f.Block {
			if b, msg := m.blockOfName(f.BlockTypeName, parent); b != nil {
				f.Nested = b.asProperties()
				if msg != "" {
					if f.ParseErrors == nil {
						f.ParseErrors = []string{}
					}
					f.ParseErrors = append(f.ParseErrors, msg)
				}
			} else {
				if b2, _ := m.blockOfName(f.Name, parent); b2 != nil {
					if f.ParseErrors == nil {
						f.ParseErrors = []string{}
					}
					f.ParseErrors = append(f.ParseErrors, fmt.Sprintf("misspell of name from `%s` to `%s`", f.Name, f.BlockTypeName))
				} else {
					if f.ParseErrors == nil {
						f.ParseErrors = []string{}
					}
					f.ParseErrors = append(f.ParseErrors, fmt.Sprintf("block `%s` not defined in documentation", f.Name))
				}
			}
		}
	}

	for _, f := range m.fields {
		fillField(f, "")
	}

	for _, b := range m.blocks {
		for _, f := range b.Fields {
			fillField(f, b.Name)
		}
	}
}

func (m *mark) blockOfName(name string, parent string) (*markBlock, string) {
	var res []*markBlock
	for i := range m.blocks {
		b := &m.blocks[i]
		for _, n2 := range b.Names {
			if n2 == name {
				res = append(res, b)
			}
		}
	}

	if len(res) == 0 {
		return nil, ""
	}

	if parent != "" {
		for _, item := range res {
			if item.Of == parent {
				return item, ""
			}
		}
	}

	var msg string
	if len(res) > 1 {
		// Check if these are actual duplicate block definitions or just shared references
		uniqueDefinitions := make(map[string]*markBlock)
		for _, block := range res {
			key := fmt.Sprintf("%s:%d", block.Name, len(block.Fields))
			if existing, exists := uniqueDefinitions[key]; exists {
				if !blocksHaveSameDefinition(existing, block) {
					msg = fmt.Sprintf("duplicate block exists as name `%s`", name)
					break
				}
			} else {
				uniqueDefinitions[key] = block
			}
		}
	}
	return res[0], msg
}

// blocksHaveSameDefinition checks if two blocks have the same definition/content
func blocksHaveSameDefinition(b1, b2 *markBlock) bool {
	if len(b1.Fields) != len(b2.Fields) {
		return false
	}

	for i, f1 := range b1.Fields {
		if i >= len(b2.Fields) {
			return false
		}
		f2 := b2.Fields[i]
		if f1.Name != f2.Name || f1.Required != f2.Required {
			return false
		}
	}

	return true
}

// parseMarkdownSection is the main entry point for parsing a markdown section
func parseMarkdownSection(content []string) *Properties {
	// Join lines back into a single string for parsing
	fullContent := strings.Join(content, "\n")

	mark := newMarkFromString(fullContent)

	result := NewProperties()

	// Copy top-level fields
	for name, field := range mark.fields {
		result.Objects[name] = field
		result.Names = append(result.Names, name)
	}

	// Copy block definitions
	for _, block := range mark.blocks {
		if blockProp := convertBlockToProperty(&block); blockProp != nil {
			result.BlockDefinitions[block.Name] = blockProp
		}
	}

	return result
}

func convertBlockToProperty(block *markBlock) *Property { // TODO: Check if it's necessary
	prop := &Property{
		Name:    block.Name,
		Block:   true,
		Line:    block.HeadLine,
		Nested:  block.asProperties(),
	}

	return prop
}

// ===== Field parsing helpers =====
// TODO: Consider move this to separate files, currently it's quite hard to maintain and understand the code :(

var fieldReg = regexp.MustCompile("^[*-] *`(.*?)`" + ` +\- +(\(Required\)|\(Optional\))? ?(.*)`)
var codeReg = regexp.MustCompile("`([^`]+)`")
var blockHeadReg = regexp.MustCompile("^(an?|An?|The)[^`]+(`[a-zA-Z0-9_]+`[, and]*)+.*blocks?.*$")
var DefaultsReg = regexp.MustCompile("[.,?;](?: *[Tt]he)? *[Dd]efaults?[^`'\".]+(?:to|is) ('[^']+'|`[^`]+`|\"[^\"]+\")[ .,]?")
var ForceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]*(\.|$)`)
var partForceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]* created when [^.]*(\.|$)`)
var blockPropRegs = []*regexp.Regexp{
	regexp.MustCompile("(?:[Oo]ne|[Ee]ach|more(?: \\(.*\\))?|[Tt]he|as|of|[Aa]n?) ['\"`]([^ ]+)['\"`] (?:block|object)[^.]+(?:below|above)"),
}
var blockTypeReg = blockPropRegs[0]

func getDefaultValue(line string) string {
	if vals := DefaultsReg.FindStringSubmatch(line); len(vals) > 0 {
		if val := vals[1]; len(val) > 2 {
			return val[1 : len(val)-1]
		}
	}
	return ""
}

func isForceNew(line string) bool {
	if ForceNewReg.MatchString(line) && !partForceNewReg.MatchString(line) {
		return true
	}
	return false
}

func extractFieldFromLine(line string) *Property {
	field := &Property{
		Content: line,
	}

	if defaultVal := getDefaultValue(line); defaultVal != "" {
		field.DefaultValue = defaultVal
	}
	field.ForceNew = isForceNew(line)

	res := fieldReg.FindStringSubmatch(line)
	if len(res) <= 1 || res[1] == "" {
		field.Name = util.FirstCodeValue(line)
		if field.ParseErrors == nil {
			field.ParseErrors = []string{}
		}
		field.ParseErrors = append(field.ParseErrors, "no field name found")
		return field
	}
	field.Name = res[1]
	if field.Name == "" {
		log.Printf("field name is empty")
	}
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
	if len(res) > 3 {
		// extract enums from code part
		// from possible value to first '.'
		// skip if there are more than one sep exists
		// do not check the possible part
		if sepIdx := possibleValueSep(line); sepIdx > 0 {
			subStr := line[sepIdx:]
			field.EnumStart = sepIdx
			// end with dot may not work in values like `7.2` ....
			// should be . not in ` mark
			// Possible values are `a`, `b`, `a.b` and `def`.
			pointEnd := strings.Index(subStr, ".")
			if pointEnd < 0 {
				pointEnd = len(subStr)
			}
			enumIndex := codeReg.FindAllStringIndex(subStr, -1)
			for _, val := range enumIndex {
				start, end := val[0], val[1]
				if pointEnd > start && pointEnd < end {
					// point inside the code block
					if pointEnd = strings.Index(subStr[end:], "."); pointEnd < 0 {
						pointEnd = len(subStr)
					} else {
						pointEnd += end
					}
				}
				if pointEnd < start {
					break
				}
				enums = append(enums, strings.Trim(subStr[start:end], "`'\""))
				field.EnumEnd = sepIdx + end
			}
			// breaks if there are more than 1 possible value
			if sepIdx = possibleValueSep(line[sepIdx+1:]); sepIdx >= 0 {
				// field.Skip = true // TODO add skip
				if field.ParseErrors == nil {
					field.ParseErrors = []string{}
				}
				field.ParseErrors = append(field.ParseErrors, "multiple possible values sections")
			}
		}
		if len(enums) == 0 && strings.Index(res[3], "`") > 0 {
			guessValues := codeReg.FindAllString(res[3], -1)
			field.GuessEnums = setGuessEnums(guessValues)
		}
	}
	field.PossibleValues = enums  // TODO: Change to AddEnum?
	return field
}

func setGuessEnums(values []string) []string {
	hys := make(map[string]struct{}, len(values))
	var res []string
	for _, val := range values {
		val = strings.Trim(val, "`\"'")
		if _, ok := hys[val]; !ok {
			hys[val] = struct{}{}
			res = append(res, val)
		}
	}
	return res
}

func extractBlockNames(line string) []string {
	if blockHeadReg.MatchString(line) {
		idx := strings.Index(line, "block")
		names := codeReg.FindAllString(line[:idx], -1)
		for idx, val := range names {
			names[idx] = strings.Trim(val, "`'")
		}
		return names
	}
	return nil
}

func isBlockHead(line string) bool {
	return blockHeadReg.MatchString(line)
}

func guessBlockProperty(line string) bool {
	for _, reg := range blockPropRegs {
		if reg.MatchString(line) {
			return true
		}
	}
	return strings.Contains(line, "A block to")
}

func newFieldFromLine(line string) *Property {
	f := extractFieldFromLine(line)
	if guessBlockProperty(line) {
		// extract real block type
		f.BlockTypeName = f.Name
		if match := blockTypeReg.FindAllStringSubmatchIndex(strings.ToLower(line), -1); len(match) > 0 {
			f.BlockTypeName = line[match[0][2]:match[0][3]]
		}
		f.Block = true
	}
	return f
}
