# Design Document: Markdown-to-Struct Parsing for document-fmt

## Overview

This document proposes implementing structured markdown parsing capabilities in `document-fmt`, similar to those already present in `document-lint`. The goal is to enable `document-fmt` to parse Terraform provider documentation markdown files into structured data that can be used for advanced validation, formatting, and scaffolding operations.

## Current State Analysis

### document-lint Architecture

**Strengths:**
- **Rich Parsing**: Comprehensive markdown parsing with support for fields, blocks, enums, and metadata
- **Structured Data Model**: Well-defined data structures (`Field`, `Properties`, `Block`) for representing documentation content
- **Field Extraction**: Advanced regex-based field parsing with support for required/optional flags, default values, and possible values
- **Block Handling**: Support for nested blocks and block relationships ("block of", "within" constructs)
- **Position Tracking**: Tracks where content appears (Arguments, Attributes, Timeouts, etc.)

**Key Components:**
1. **Parser (`md/parse_md.go`)**: Line-by-line markdown parsing with type classification
2. **Model (`model/resource_doc.go`)**: Data structures for representing parsed content
3. **Field Extractor (`md/resource_doc.go`)**: Regex-based field parsing with metadata extraction
4. **Structure Builder**: Constructs hierarchical field/block relationships

### document-fmt Architecture

**Current Approach:**
- **Section-based Parsing**: Splits documents into sections (Arguments, Attributes, Examples, etc.)
- **Template-driven**: Focus on formatting and scaffolding rather than content analysis
- **Simple Property Model**: Basic property representation without advanced metadata
- **Validator Integration**: Rule-based validation system

**Limitations:**
- No detailed field-level parsing
- Limited understanding of markdown structure
- Cannot extract field metadata (required/optional, enums, defaults)
- No support for block relationships

## Proposed Design

### 1. Enhanced Data Model

Add new data structures to `data/` package to represent parsed markdown content:

```go
// Enhanced field representation with document-lint capabilities
type DocumentField struct {
    Name         string
    Path         string              // xpath-like path (a.b.c)
    Line         int                 // source line number
    Type         FieldType           // attribute or block
    Position     PositionType        // Arguments, Attributes, Timeouts
    Required     RequiredType        // Required, Optional, Computed
    Default      string              // default value
    ForceNew     bool               // force new flag
    Content      string             // original markdown line
    Description  string             // field description
    
    // Enum support
    PossibleValues []string
    
    // Block support
    Nested       *DocumentProperties // nested fields for blocks
    BlockType    string             // block type name
    
    // Metadata
    FormatErrors []string           // parsing errors
}

type DocumentProperties struct {
    Fields map[string]*DocumentField
    Order  []string                 // maintain field order
}

type DocumentBlock struct {
    Name         string
    Names        []string           // alternative names
    Type         string             // block type
    Of           string             // "block of" target
    Position     PositionType
    Line         int
    Fields       *DocumentProperties
}
```

### 2. Enhanced Markdown Parser

Create new parser in `markdown/` package:

```go
// markdown/document_parser.go
type DocumentParser struct {
    content    string
    lines      []string
    sections   []Section          // existing section parsing
    
    // New structured parsing
    fields     *DocumentProperties
    blocks     []*DocumentBlock
    metadata   *DocumentMetadata
}

type ParseResult struct {
    Fields     *DocumentProperties
    Blocks     []*DocumentBlock
    Metadata   *DocumentMetadata
    Errors     []ParseError
}

func (p *DocumentParser) ParseStructured() (*ParseResult, error) {
    // Implementation combining section-based and line-based parsing
}
```

### 3. Parser Implementation Strategy

#### Phase 1: Line-by-Line Classification
Adapt document-lint's line classification approach:

```go
type LineType int

const (
    LineHeader1 LineType = iota
    LineHeader2
    LineHeader3
    LineField
    LineBlockHead
    LineExample
    LineNote
    LinePlainText
    LineSeparator
)

type ParsedLine struct {
    Number   int
    Content  string
    Type     LineType
    Metadata map[string]interface{}
}
```

#### Phase 2: Field Extraction
Port document-lint's field extraction logic:

```go
// Regex patterns for field parsing (from document-lint)
var (
    fieldRegex       = regexp.MustCompile(`\* ` + "`" + `([^`]+)` + "`")
    blockHeadRegex   = regexp.MustCompile(`A (\w+) block supports`)
    possibleValRegex = regexp.MustCompile(`possible value|must be one of|valid option`)
)

func extractFieldFromLine(line string) (*DocumentField, error) {
    // Port document-lint field extraction logic
}
```

#### Phase 3: Structure Building
Build hierarchical structures from parsed lines:

```go
func (p *DocumentParser) buildStructure(lines []*ParsedLine) (*ParseResult, error) {
    var currentPosition PositionType
    var currentBlock *DocumentBlock
    
    for _, line := range lines {
        switch line.Type {
        case LineHeader2, LineHeader3:
            currentPosition = determinePosition(line.Content)
            if currentBlock != nil {
                // finalize current block
            }
        case LineField:
            field := extractFieldFromLine(line.Content)
            field.Position = currentPosition
            // add to current block or root fields
        case LineBlockHead:
            // start new block
            currentBlock = extractBlockFromLine(line.Content)
        }
    }
}
```

### 4. Integration with Existing Systems

#### TerraformNodeData Enhancement
Extend existing data structure:

```go
type TerraformNodeData struct {
    // ... existing fields ...
    
    // New structured parsing results
    ParsedDocument   *ParseResult
    DocumentFields   *DocumentProperties  // replaces DocumentArguments/DocumentAttributes
    DocumentBlocks   []*DocumentBlock
}
```

#### Validator Integration
Enable advanced validation rules:

```go
// rule/field_validation.go
type FieldValidationRule struct {
    // Rules that can now access structured field data
}

func (r *FieldValidationRule) Validate(nodeData *TerraformNodeData, fix bool) []error {
    // Access nodeData.ParsedDocument for detailed validation
    for _, field := range nodeData.DocumentFields.Fields {
        // Validate field metadata, enums, requirements, etc.
    }
}
```

### 5. Migration Strategy

#### Phase 1: Parallel Implementation
- Add new parsing system alongside existing section-based parsing
- Maintain backward compatibility
- Add feature flag to enable structured parsing

#### Phase 2: Gradual Migration
- Update validators to use structured data when available
- Enhance rules to leverage field metadata
- Add new validation capabilities

#### Phase 3: Full Integration
- Replace simple property model with structured model
- Remove legacy parsing code
- Optimize performance

### 6. Benefits

#### Enhanced Validation
- **Field-level validation**: Check required/optional consistency
- **Enum validation**: Verify possible values documentation
- **Block validation**: Ensure block structure correctness
- **Cross-reference validation**: Compare docs against schema

#### Improved Formatting
- **Intelligent formatting**: Format based on field types and metadata
- **Consistent ordering**: Maintain proper field ordering
- **Auto-correction**: Fix common documentation issues

#### Better Scaffolding
- **Schema-aware scaffolding**: Generate docs from Terraform schema
- **Metadata preservation**: Maintain field descriptions and examples
- **Block structure generation**: Auto-generate block documentation

### 7. Implementation Plan

#### Milestone 1: Core Parser (2-3 weeks)
- [ ] Implement line classification system
- [ ] Port field extraction logic from document-lint
- [ ] Create enhanced data structures
- [ ] Add basic structure building

#### Milestone 2: Integration (1-2 weeks)
- [ ] Integrate with existing TerraformNodeData
- [ ] Update document parsing pipeline
- [ ] Add feature flag for new parser

#### Milestone 3: Validation Enhancement (2-3 weeks)
- [ ] Create field-aware validation rules
- [ ] Add enum and metadata validation
- [ ] Implement cross-reference checks

#### Milestone 4: Testing & Optimization (1-2 weeks)
- [ ] Comprehensive testing
- [ ] Performance optimization
- [ ] Documentation updates

### 8. Risks and Mitigation

#### Complexity Risk
- **Risk**: Adding parsing complexity to document-fmt
- **Mitigation**: Modular design, maintain clear separation of concerns

#### Performance Risk
- **Risk**: Slower document processing
- **Mitigation**: Optimize parsing algorithms, add caching

#### Compatibility Risk
- **Risk**: Breaking existing workflows
- **Mitigation**: Parallel implementation, feature flags, thorough testing

### 9. Success Metrics

- **Parsing Accuracy**: >95% successful field extraction from existing docs
- **Performance**: <20% increase in processing time
- **Validation Coverage**: 3x more validation rules enabled
- **Developer Experience**: Reduced manual documentation errors

## Conclusion

Implementing structured markdown parsing in document-fmt will significantly enhance its capabilities while maintaining compatibility with existing workflows. The proposed design leverages proven approaches from document-lint while integrating seamlessly with document-fmt's architecture.

The phased implementation approach minimizes risk while delivering incremental value. The enhanced validation and formatting capabilities will improve documentation quality and developer productivity across the Terraform provider ecosystem.