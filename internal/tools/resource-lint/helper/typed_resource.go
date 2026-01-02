package helper

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	pluginschema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-provider-azurerm/internal/sdk"
)

// TypedResourceInfo represents gathered information about a typed Terraform resource
type TypedResourceInfo struct {
	ResourceTypeName      string // Go type name (e.g., ManagementGroupPolicyDefinitionResource)
	TerraformResourceType string // Terraform type name (e.g., azurerm_management_group_policy_definition)
	ModelName             string
	ModelStruct           *ast.StructType
	ArgumentsFunc         *ast.FuncDecl
	AttributesFunc        *ast.FuncDecl
	CreateFunc            *ast.FuncDecl
	ReadFunc              *ast.FuncDecl
	UpdateFunc            *ast.FuncDecl
	DeleteFunc            *ast.FuncDecl
	TypesInfo             *types.Info
	ModelFieldToTFSchema  map[string]string // model struct field name -> tfschema tag name
	RuntimeSchema         *pluginschema.Resource
	SchemaError           error
}

// NewTypedResourceInfo creates a TypedResourceInfo by parsing a typed resource from file
func NewTypedResourceInfo(resourceTypeName string, file *ast.File, info *types.Info) *TypedResourceInfo {
	result := &TypedResourceInfo{
		ResourceTypeName:     resourceTypeName,
		TypesInfo:            info,
		ModelFieldToTFSchema: make(map[string]string),
	}

	// Single pass: collect all information from file.Decls
	structs := make(map[string]*ast.StructType)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			// Collect struct definitions
			for _, spec := range d.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					if structType, ok := typeSpec.Type.(*ast.StructType); ok {
						structs[typeSpec.Name.Name] = structType
					}
				}
			}

		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				continue
			}

			recvType := GetReceiverTypeName(d.Recv.List[0].Type)
			if recvType != resourceTypeName {
				continue
			}

			// Collect methods by name
			switch d.Name.Name {
			case "ResourceType":
				// Extract resource type from: return "azurerm_..." or return ConstantName
				if d.Body != nil {
					ast.Inspect(d.Body, func(n ast.Node) bool {
						ret, ok := n.(*ast.ReturnStmt)
						if !ok || len(ret.Results) == 0 {
							return true
						}

						switch expr := ret.Results[0].(type) {
						case *ast.BasicLit:
							// Direct string literal: return "azurerm_..."
							result.TerraformResourceType = strings.Trim(expr.Value, `"`)
							return false
						case *ast.Ident:
							// Use TypesInfo to get the constant value
							if tv, ok := info.Types[expr]; ok {
								if tv.Value != nil {
									result.TerraformResourceType = strings.Trim(tv.Value.String(), `"`)
									return false
								}
							}
						}
						return true
					})
				}
			case "ModelObject":
				// Extract model type name from: return &ModelName{}
				if d.Body != nil {
					ast.Inspect(d.Body, func(n ast.Node) bool {
						ret, ok := n.(*ast.ReturnStmt)
						if !ok || len(ret.Results) == 0 {
							return true
						}
						// Match &ModelName{} pattern
						if unaryExpr, ok := ret.Results[0].(*ast.UnaryExpr); ok {
							if compLit, ok := unaryExpr.X.(*ast.CompositeLit); ok {
								if ident, ok := compLit.Type.(*ast.Ident); ok {
									result.ModelName = ident.Name
									return false
								}
							}
						}
						return true
					})
				}

			case "Arguments":
				result.ArgumentsFunc = d
			case "Attributes":
				result.AttributesFunc = d
			case "Create":
				result.CreateFunc = d
			case "Read":
				result.ReadFunc = d
			case "Update":
				result.UpdateFunc = d
			case "Delete":
				result.DeleteFunc = d
			}
		}
	}

	// Resolve model struct from collected structs
	if result.ModelName != "" {
		if modelStruct, ok := structs[result.ModelName]; ok {
			result.ModelStruct = modelStruct
			result.parseModelStructInternal(modelStruct)
		}
	}

	// Load runtime schema
	result.loadRuntimeSchema()

	return result
}

// parseModelStructInternal is the internal implementation for parsing model struct
func (info *TypedResourceInfo) parseModelStructInternal(modelStruct *ast.StructType) {
	for _, field := range modelStruct.Fields.List {
		if field.Tag == nil {
			continue
		}

		tagValue := strings.Trim(field.Tag.Value, "`")
		if !strings.Contains(tagValue, "tfschema:") {
			continue
		}

		// Extract tfschema tag value
		parts := strings.Split(tagValue, `"`)
		if len(parts) >= 2 {
			tfschemaName := parts[1]
			if len(field.Names) > 0 {
				// Map: struct field name -> tfschema name
				info.ModelFieldToTFSchema[field.Names[0].Name] = tfschemaName
			}
		}
	}
}

// GetReceiverTypeName extracts the type name from a method receiver
func GetReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	}
	return ""
}

// IsResourceWithUpdateInterface checks if a type implements sdk.ResourceWithUpdate
func IsResourceWithUpdateInterface(expr ast.Expr) bool {
	selExpr, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkgIdent, ok := selExpr.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "sdk" {
		return false
	}

	return selExpr.Sel.Name == "ResourceWithUpdate"
}

func (r *TypedResourceInfo) loadRuntimeSchema() {
	if r.TerraformResourceType == "" {
		r.SchemaError = fmt.Errorf("terraform resource type is empty")
		return
	}

	for _, service := range GetTypedServices() {
		for _, res := range service.Resources() {
			if res.ResourceType() == r.TerraformResourceType {
				wrapper := sdk.NewResourceWrapper(res)
				r.RuntimeSchema, r.SchemaError = wrapper.Resource()
				return
			}
		}
	}

	r.SchemaError = fmt.Errorf("resource not found: %s", r.TerraformResourceType)
}

// GetUpdatableProperties returns updatable properties from runtime schema
func (r *TypedResourceInfo) GetUpdatableProperties() (map[string]string, error) {
	result := make(map[string]string)

	if r.SchemaError != nil {
		return nil, r.SchemaError
	}

	if r.RuntimeSchema == nil || r.RuntimeSchema.Schema == nil {
		return result, nil
	}

	for name, elem := range r.RuntimeSchema.Schema {
		if isUpdatableSchemaProperty(elem) {
			// Find corresponding model field name
			modelFieldName := ""
			for field, tfschema := range r.ModelFieldToTFSchema {
				if tfschema == name {
					modelFieldName = field
					break
				}
			}
			result[name] = modelFieldName
		}
	}

	return result, nil
}

// isUpdatableSchemaProperty checks if a schema property is updatable
func isUpdatableSchemaProperty(elem *pluginschema.Schema) bool {
	// Skip Computed fields, including O+C, as some of those are not updatable
	if elem.Computed {
		return false
	}

	// Skip ForceNew fields
	if elem.ForceNew {
		return false
	}

	// Must be settable (Optional or Required)
	return elem.Optional || elem.Required
}
