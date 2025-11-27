// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/data/models"
)

// CheckIssue represents a single validation issue
type CheckIssue struct {
	LineNum   int
	Key       string
	Message   string
	FixLine   string
	Line      string
	DocProp   *models.DocumentProperty
	CheckType string // for categorizing issues
}

func (ci *CheckIssue) Error() string {
	var result string
	result = ci.Message

	if ci.FixLine != "" && ci.Line != "" && ci.FixLine != ci.Line {
		result += "\n"
		result += "     " + ci.Line + "\n"
		result += "  => " + ci.FixLine
	}

	return result
}

// PropertyCheckContext holds the context for marker checks
type PropertyCheckContext struct {
	FullPath         string
	SchemaProperty   *models.SchemaProperty
	DocProperty      *models.DocumentProperty
	BlockDefinitions map[string]*models.DocumentProperty
}
