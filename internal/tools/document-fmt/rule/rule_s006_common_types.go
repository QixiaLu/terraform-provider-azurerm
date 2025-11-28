// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"strings"

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
		line := ci.Line
		result += "     " + line
		if !strings.HasSuffix(line, "\n") {
			result += "\n"
		}
		result += "  => " + ci.FixLine
		if !strings.HasSuffix(ci.FixLine, "\n") {
			result += "\n"
		}
	}

	return result
}

// PropertyCheckContext holds the context for marker checks
type PropertyCheckContext struct {
	FullPath         string
	SchemaProperty   *models.SchemaProperty
	DocProperty      *models.DocumentProperty
	BlockDefinitions map[string]*models.DocumentProperty
	FileName         string
}
