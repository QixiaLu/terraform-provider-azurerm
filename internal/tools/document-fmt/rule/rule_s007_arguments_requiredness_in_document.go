package rule

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/markdown"
)

type S007 struct{}

var _ Rule = new(S007)

func (s S007) ID() string {
	return "S007"
}

func (s S007) Name() string {
	return "Properties Requiredness in Document"
}

func (s S007) Description() string {
	return "Determines whether all properties are correctly marked as Required or Optional in documentation"
}

func (s S007) Run(d *data.TerraformNodeData, fix bool) []error {
	var errs []error

	if !d.Document.Exists {
		return errs
	}

	// TODO: complete data source check
	if d.Type == data.ResourceTypeData {
		return errs
	}

	if d.SchemaProperties == nil || d.DocumentArguments == nil {
		return errs
	}

	errs = append(errs, s.checkRequiredness(d, "", d.SchemaProperties, d.DocumentArguments, d.DocumentAttributes, d.DocumentArguments.BlockDefinitions, fix)...)

	return errs
}

// getExpectedLine adds the required/optional marker to a line that's missing it
func (s S007) getExpectedLine(line, marker string) string {
	// Try to add after the first " - "
	if idx := strings.Index(line, " - "); idx > 0 {
		return line[:idx+3] + marker + " " + line[idx+3:]
	}
	// If no dash, add after the second backtick
	firstBacktick := strings.Index(line, "`")
	if firstBacktick >= 0 {
		secondBacktick := strings.Index(line[firstBacktick+1:], "`")
		if secondBacktick >= 0 {
			idx := firstBacktick + 1 + secondBacktick + 1
			return line[:idx] + " " + marker + line[idx:]
		}
	}
	return line
}

// replaceRequiredness replaces one requiredness marker with another
func (s S007) replaceRequiredness(line, from, to string) string {
	if strings.Contains(line, from) {
		return strings.Replace(line, from, to, 1)
	} else {
		// add after the first -
		if idx := strings.Index(line, " - "); idx > 0 {
			line = line[:idx+3] + to + " " + line[idx+3:]
		} else {
			// no dash add after second `
			idx = strings.Index(line, "`")
			idx += strings.Index(line[idx+1:], "`") + 1
			line = line[:idx+1] + " " + to + line[idx+1:]
		}
	}
	return line
}

// removeRequiredness removes (Required) or (Optional) markers from a line
func (s S007) removeRequiredness(line string) string {
	from := "(Required)"
	to := "(Optional)"

	var idx, size int
	if idx = strings.Index(line, from); idx > 0 {
		size = len(from)
	} else if idx = strings.Index(line, to); idx > 0 {
		size = len(to)
	}

	if idx > 0 {
		if idx > 0 && line[idx-1] == ' ' && idx+size < len(line) && line[idx+size] == ' ' {
			idx -= 1
			size += 1
		}
		line = line[:idx] + line[idx+size:]
	}

	return line
}

// checkRequiredness checks if schema properties have correct Required/Optional markings in documentation
func (s S007) checkRequiredness(d *data.TerraformNodeData, parentPath string, schema *models.SchemaProperties, docArgs *models.DocumentProperties, docAttrs *models.DocumentProperties, blockDefinitions map[string]*models.DocumentProperty, fix bool) []error {
	errs := make([]error, 0)
	resourceType := d.Name

	if schema == nil || docArgs == nil {
		return errs
	}

	for name, schemaProperty := range schema.Objects {
		// TDOO: Add a rule check that Attr shouldnt have optional + required prefix
		if !schemaProperty.Optional && schemaProperty.Computed {
			continue
		}
		if name == "id" {
			continue
		}
		if schemaProperty.Deprecated {
			continue
		}

		fullPath := name
		if parentPath != "" {
			fullPath = parentPath + "." + name
		}

		if isSkipProp(resourceType, fullPath) {
			continue
		}

		docProperty := docArgs.Objects[name]
		if docProperty == nil {
			continue
		}

		if schemaProperty.Required {
			if !docProperty.Required {
				cleanContent := strings.TrimRight(docProperty.Content, "\n")
				expected := s.replaceRequiredness(cleanContent, "(Required)", "(Optional)")
				errs = append(errs, fmt.Errorf("%s: `%s` should be Required in %s\n%s\n=>%s",
					IdAndName(s), fullPath, d.Document.Path, cleanContent, expected))  /// Only keep file name

				if fix {
					s.fixPropertyLine(d, docProperty.Line, expected, false)
				}
			}
		} else if schemaProperty.Optional {
			if !docProperty.Optional {
				cleanContent := strings.TrimRight(docProperty.Content, "\n")
				expected := s.replaceRequiredness(cleanContent, "(Optional)", "(Required)")
				errs = append(errs, fmt.Errorf("%s: `%s` should be Optional in %s\n%s\n=>%s",
					IdAndName(s), fullPath, d.Document.Path, cleanContent, expected))   /// ONly keep file name

				if fix {
					s.fixPropertyLine(d, docProperty.Line, expected, false)
				}
			}
		}

		if schemaProperty.Nested != nil && len(schemaProperty.Nested.Objects) > 0 {
			if docProperty.Nested != nil && len(docProperty.Nested.Objects) > 0 {
				var nestedDocAttrs *models.DocumentProperties
				if docAttrs != nil {
					if attrProp := docAttrs.Objects[name]; attrProp != nil {
						nestedDocAttrs = attrProp.Nested
					}
				}

				var nestedDocArgs *models.DocumentProperties
				if docProperty.BlockTypeName != docProperty.Name {
					linkedDocProperty := blockDefinitions[docProperty.BlockTypeName]
					if linkedDocProperty != nil && linkedDocProperty.Nested != nil {
						nestedDocArgs = linkedDocProperty.Nested
					} else {
						nestedDocArgs = docProperty.Nested
					}
				} else {
					nestedDocArgs = docProperty.Nested
				}

				errs = append(errs, s.checkRequiredness(d, fullPath, schemaProperty.Nested, nestedDocArgs, nestedDocAttrs, blockDefinitions, fix)...)
			}
		}
	}

	return errs
}

func (s S007) fixPropertyLine(d *data.TerraformNodeData, lineIndex int, expected string, isAttributesSection bool) {
	if d.Document == nil {
		return
	}

	// Find the correct section (Arguments or Attributes)
	for _, section := range d.Document.Sections {
		var isTargetSection bool

		if isAttributesSection {
			_, isTargetSection = section.(*markdown.AttributesSection)
		} else {
			_, isTargetSection = section.(*markdown.ArgumentsSection)
		}

		if !isTargetSection {
			continue
		}

		content := section.GetContent()

		if lineIndex < 0 || lineIndex >= len(content) {
			return
		}

		content[lineIndex] = expected
		section.SetContent(content)
		d.Document.HasChange = true
		return
	}
}
