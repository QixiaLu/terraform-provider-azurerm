package data

import (
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/types"
)

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
)

type Properties struct {
	Names   []string // Only really relevant to the documentation, could be used to track ordering in docs to compare against ordering we want
	Objects map[string]*Property
}

type Property struct {
	// Basic attributes
	Name        string
	Type        string
	Description string
	Required    bool
	Optional    bool
	Computed    bool
	ForceNew    bool
	Deprecated  bool

	PossibleValues []string
	DefaultValue   interface{} // Default value can be many types, TODO: convert func to cast from interface{} to string and change this field type to string

	// Block related attributes
	Nested          *Properties
	Block           bool
	BlockHasSection bool // TODO?

	// List or map related attributes
	NestedType string

	// Documentation related attributes
	AdditionalLines []string // Tracks any lines from docs beyond initial property, e.g. notes
	Count           int      // Property count, for doc parsing to detect duplicate entries

	Path           string       // xpath-like path (a.b.c)
	Line           int          // source line number in documentation
	Position       PositionType // Arguments, Attributes, Timeouts etc.
	Content        string       // original markdown line content
	EnumStart      int          // start position of enum values in content
	EnumEnd        int          // end position of enum values in content
	ParseErrors    []string     // errors encountered during parsing
	BlockTypeName  string       // block type name (may differ from field name)
	SameNameAttr   *Property    // reference to same-named field in different position
	GuessEnums     []string     // guessed enum values from code blocks
}

func NewProperties() *Properties {
	return &Properties{
		Names:   make([]string, 0),
		Objects: make(map[string]*Property),
	}
}

// AddProperty adds a property to the collection
func (props *Properties) AddProperty(p *Property) {
	if props == nil {
		return
	}
	if p == nil || p.Name == "" {
		return
	}

	// TODO: Fix this, for block, there should already be a link, which is not duplication
	// Check if property already exists (duplicate detection)
	if existing, exists := props.Objects[p.Name]; exists {
		// Property exists in same section - increment count and track as duplicate
		existing.Count++
		// Store parse error for duplicate detection
		if existing.ParseErrors == nil {
			existing.ParseErrors = []string{}
		}
		existing.ParseErrors = append(existing.ParseErrors, "duplicate field in same section")
		return
	}

	props.Names = append(props.Names, p.Name)
	props.Objects[p.Name] = p
}

// FindProperty searches for a property by name recursively
func (props *Properties) FindProperty(name string) *Property {
	if props == nil {
		return nil
	}

	for _, prop := range props.Objects {
		if result := prop.FindProperty(name); result != nil {
			return result
		}
	}
	return nil
}

// FindAllSubBlocks finds all sub-blocks with the given name
func (props *Properties) FindAllSubBlocks(name string) []*Property {
	if props == nil {
		return nil
	}

	var result []*Property
	for _, prop := range props.Objects {
		result = append(result, prop.FindAllSubBlocks(name, true)...)
	}

	// If no blocks found, try non-block properties
	if len(result) == 0 {
		for _, prop := range props.Objects {
			result = append(result, prop.FindAllSubBlocks(name, false)...)
		}
	}
	return result
}

// HasCircularReference checks if there are circular references in the properties
func (props *Properties) HasCircularReference() string {
	if props == nil {
		return ""
	}

	for name, prop := range props.Objects {
		if prop.Block && prop.HasCircularReference(nil) {
			return name
		}
	}
	return ""
}

// Merge merges properties from another Properties collection
func (props *Properties) Merge(other *Properties) {
	if props == nil || other == nil {
		return
	}

	for name, prop := range other.Objects {
		if existing, exists := props.Objects[name]; exists {
			// Property exists, set as same name reference
			existing.SameNameAttr = prop
		} else {
			// Add new property
			props.Names = append(props.Names, name)
			props.Objects[name] = prop
		}
	}
}

func (p *Property) String() string {
	return "TODO"
}

// AddEnum adds enum values to PossibleValues while avoiding duplicates
func (p *Property) AddEnum(values ...string) {
	existingMap := make(map[string]bool)
	for _, v := range p.PossibleValues {
		existingMap[v] = true
	}

	for _, value := range values {
		trimmed := strings.Trim(value, "`\"'")
		if trimmed != "" && !existingMap[trimmed] {
			p.PossibleValues = append(p.PossibleValues, trimmed)
			existingMap[trimmed] = true
		}
	}
}

// SetGuessEnums sets guess enum values after removing duplicates
func (p *Property) SetGuessEnums(values []string) {
	seen := make(map[string]bool)
	var result []string
	for _, val := range values {
		val = strings.Trim(val, "`\"'")
		if val != "" && !seen[val] {
			seen[val] = true
			result = append(result, val)
		}
	}
	p.GuessEnums = result
}

// AddSubProperty adds a nested property
func (p *Property) AddSubProperty(sub *Property) {
	if p.Nested == nil {
		p.Nested = NewProperties()
	}
	p.Nested.Names = append(p.Nested.Names, sub.Name)
	p.Nested.Objects[sub.Name] = sub
}

// FindProperty recursively searches for a property by name
func (p *Property) FindProperty(name string) *Property {
	if p.Name == name {
		return p
	}
	if p.Nested != nil {
		for _, nested := range p.Nested.Objects {
			if result := nested.FindProperty(name); result != nil {
				return result
			}
		}
	}
	return nil
}

// FindAllSubBlocks finds all sub-blocks with the given name
func (p *Property) FindAllSubBlocks(name string, needBlock bool) []*Property {
	var result []*Property

	// Check if this property itself matches
	if p.Block && p.BlockTypeName == name {
		result = append(result, p)
		return result
	}
	if !needBlock && p.BlockTypeName == "" && p.Name == name {
		result = append(result, p)
		return result
	}

	// Recursively search nested properties
	if p.Nested != nil {
		for _, nested := range p.Nested.Objects {
			result = append(result, nested.FindAllSubBlocks(name, needBlock)...)
		}
	}
	return result
}

// HasCircularReference checks if there's a circular reference in nested properties
func (p *Property) HasCircularReference(visited map[string]bool) bool {
	if visited == nil {
		visited = make(map[string]bool)
	}

	if visited[p.Name] {
		return true
	}

	if p.Block && p.Nested != nil {
		visited[p.Name] = true
		defer delete(visited, p.Name)

		for _, nested := range p.Nested.Objects {
			if nested.HasCircularReference(visited) {
				return true
			}
		}
	}
	return false
}

// BuildBlockStructure links block-type fields to their block definitions
func (props *Properties) BuildBlockStructure() {
	if props == nil {
		return
	}

	// Collect all block definitions (properties with Block=true and non-empty Nested)
	blockDefinitions := make(map[string]*Property)
	for name, prop := range props.Objects {
		if prop.Block && prop.Nested != nil && len(prop.Nested.Objects) > 0 {
			// This is a block definition section
			blockDefinitions[name] = prop
			// Also try with BlockTypeName if different
			if prop.BlockTypeName != "" && prop.BlockTypeName != name {
				blockDefinitions[prop.BlockTypeName] = prop
			}
		}
	}

	// Recursive function to link block fields
	var fillBlockFields func(prop *Property, parentPath string)
	fillBlockFields = func(prop *Property, parentPath string) {
		if prop.Block && (prop.Nested == nil || len(prop.Nested.Objects) == 0) {
			// This is a block-type field that needs to be linked to its definition
			blockName := prop.BlockTypeName
			if blockName == "" {
				blockName = prop.Name
			}

			// Look for the block definition
			if blockDef, exists := blockDefinitions[blockName]; exists {
				// Link the block definition's properties to this field
				if blockDef.Nested != nil {
					prop.Nested = blockDef.Nested
				}
			}
		}

		// Recursively process nested properties
		if prop.Nested != nil {
			for _, nested := range prop.Nested.Objects {
				nestedPath := prop.Name
				if parentPath != "" {
					nestedPath = parentPath + "." + prop.Name
				}
				fillBlockFields(nested, nestedPath)
			}
		}
	}

	// Process all top-level properties
	for _, prop := range props.Objects {
		fillBlockFields(prop, "")
	}
}
