package data

import (
	"strings"
)

type Properties struct {
	Names            []string // Only really relevant to the documentation, could be used to track ordering in docs to compare against ordering we want
	Objects          map[string]*Property
	BlockDefinitions map[string]*Property // Separate storage for block definitions ("A `name` block supports:")
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
	BlockSection    *Property

	// List or map related attributes
	NestedType string

	// Documentation related attributes
	AdditionalLines []string // Tracks any lines from docs beyond initial property, e.g. notes
	Count           int      // Property count, for doc parsing to detect duplicate entries
	Path          string       // xpath-like path (a.b.c)
	Line          int          // source line number in documentation
	Content       string       // original markdown line content
	EnumStart     int          // start position of enum values in content
	EnumEnd       int          // end position of enum values in content
	ParseErrors   []string     // errors encountered during parsing
	BlockTypeName string       // block type name (may differ from field name)
	GuessEnums    []string     // guessed enum values from code blocks
}

func NewProperties() *Properties {
	return &Properties{
		Names:            make([]string, 0),
		Objects:          make(map[string]*Property),
		BlockDefinitions: make(map[string]*Property),
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

// AddBlockDefinition adds a block definition to the separate collection
// This is used for block definition sections like "A `blob_properties` block supports:"
func (props *Properties) AddBlockDefinition(blockDef *Property) {
	if props == nil {
		return
	}
	if blockDef == nil || blockDef.Name == "" {
		return
	}

	if existing, exists := props.BlockDefinitions[blockDef.Name]; exists {
		existing.Count++
		if existing.ParseErrors == nil {
			existing.ParseErrors = []string{}
		}
		existing.ParseErrors = append(existing.ParseErrors, "duplicate block definition")
		return
	}

	props.BlockDefinitions[blockDef.Name] = blockDef
}

// LinkBlockDefinitions links block-type fields to their definitions
func (props *Properties) LinkBlockDefinitions() {
	if props == nil {
		return
	}
	props.linkBlockDefinitionsWithRegistry(props.BlockDefinitions)
}

// linkBlockDefinitionsWithRegistry recursively links block fields using a global block definition registry
func (props *Properties) linkBlockDefinitionsWithRegistry(globalBlockDefs map[string]*Property) {
	if props == nil {
		return
	}

	for _, field := range props.Objects {
		if field.Block {
			// Look for corresponding block definition
			blockName := field.BlockTypeName
			if blockName == "" {
				blockName = field.Name
			}

			if blockDef, exists := globalBlockDefs[blockName]; exists {
				if blockDef.Nested != nil && len(blockDef.Nested.Objects) > 0 {
					field.Nested = blockDef.Nested
					field.Nested.linkBlockDefinitionsWithRegistry(globalBlockDefs)
				}
			} else if field.Nested == nil || len(field.Nested.Objects) == 0 {
				if field.ParseErrors == nil {
					field.ParseErrors = []string{}
				}
				field.ParseErrors = append(field.ParseErrors, "block definition not found")
			}
		}

		if field.Nested != nil {
			field.Nested.linkBlockDefinitionsWithRegistry(globalBlockDefs)
		}
	}

	for _, blockDef := range props.BlockDefinitions {
		if blockDef.Nested != nil {
			blockDef.Nested.linkBlockDefinitionsWithRegistry(globalBlockDefs)
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
