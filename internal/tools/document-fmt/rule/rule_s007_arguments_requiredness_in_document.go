package rule

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data"
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
)

// S007 validates and fixes Required/Optional markers in documentation
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

	if d.Type == data.ResourceTypeData {
		return errs
	}

	if d.SchemaProperties == nil || d.DocumentArguments == nil {
		return errs
	}

	errs = append(errs, s.checkRequiredness(d, "", d.SchemaProperties, d.DocumentArguments, d.DocumentAttributes, d.DocumentArguments.BlockDefinitions, fix)...)

	return errs
}

// checkRequiredness checks if schema properties have correct Required/Optional markings in documentation
func (s S007) checkRequiredness(d *data.TerraformNodeData, parentPath string, schema *models.SchemaProperties, docArgs *models.DocumentProperties, docAttrs *models.DocumentProperties, blockDefinitions map[string]*models.DocumentProperty, fix bool) []error {
	errs := make([]error, 0)
	resourceType := d.Name

	if schema == nil || docArgs == nil {
		return errs
	}

	for name, schemaProperty := range schema.Objects {
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

		if len(docProperty.ParseErrors) > 0 {
			continue
		}

		if schemaProperty.Required && !docProperty.Required {
			expected := s.replaceRequiredness(docProperty.Content, "(Optional)", "(Required)")
			issue := NewValidationIssue(
				s.ID(),
				s.Name(),
				fullPath,
				fmt.Sprintf("`%s` should be marked as Required", fullPath),
				d.Document.Path,
				docProperty.Content,
				expected,
			)
			errs = append(errs, issue)

			if fix {
				s.applyRequirednessFix(d, docProperty, true)
			}
		} else if schemaProperty.Optional && !docProperty.Optional {
			expected := s.replaceRequiredness(docProperty.Content, "(Required)", "(Optional)")
			issue := NewValidationIssue(
				s.ID(),
				s.Name(),
				fullPath,
				fmt.Sprintf("`%s` should be marked as Optional", fullPath),
				d.Document.Path,
				docProperty.Content,
				expected,
			)
			errs = append(errs, issue)

			if fix {
				s.applyRequirednessFix(d, docProperty, false)
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

func (s S007) applyRequirednessFix(d *data.TerraformNodeData, docProperty *models.DocumentProperty, shouldBeRequired bool) {
	if d.Document == nil {
		return
	}

	argsSection := d.Document.GetArgumentsSection()
	if argsSection == nil {
		return
	}

	content := argsSection.GetContent()
	lineIdx := docProperty.Line
	from := "(Required)"
	to := "(Optional)"
	if shouldBeRequired {
		from = "(Optional)"
		to = "(Required)"
	}

	if lineIdx >= 0 && lineIdx < len(content) {
		line := content[lineIdx]
		fixedLine := s.replaceRequiredness(line, from, to)
		content[lineIdx] = fixedLine
		argsSection.SetContent(content)
		d.Document.HasChange = true
	}
}
