package models

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

func (p *Property) String() string {
	return "TODO"
}
