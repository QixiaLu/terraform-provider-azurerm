// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"fmt"
	"strings"
)

// RequirednesChecker performs Required/Optional marker consistency checks
type RequirednesChecker struct{}

// CheckMarkers validates that Required/Optional markers in documentation match schema
// Returns issues if markers are inconsistent or missing
func (rc *RequirednesChecker) CheckMarkers(ctx *PropertyCheckContext) []*CheckIssue {
	var issues []*CheckIssue

	if ctx.DocProperty == nil || ctx.SchemaProperty == nil {
		return issues
	}

	// Check: Required marker
	if ctx.SchemaProperty.Required && !ctx.DocProperty.Required {
		issues = append(issues, &CheckIssue{
			LineNum:   ctx.DocProperty.Line,
			Key:       ctx.FullPath,
			Message:   fmt.Sprintf("S006: `%s` should be marked as (Required)", ctx.FullPath),
			FixLine:   rc.FixRequiredness(ctx.DocProperty.Content, "(Optional)", "(Required)"),
			Line:      ctx.DocProperty.Content,
			DocProp:   ctx.DocProperty,
			CheckType: "RequiredMiss",
		})
	} else if ctx.SchemaProperty.Optional && !ctx.DocProperty.Optional {
		issues = append(issues, &CheckIssue{
			LineNum:   ctx.DocProperty.Line,
			Key:       ctx.FullPath,
			Message:   fmt.Sprintf("S006: `%s` should be marked as (Optional)", ctx.FullPath),
			FixLine:   rc.FixRequiredness(ctx.DocProperty.Content, "(Required)", "(Optional)"),
			Line:      ctx.DocProperty.Content,
			DocProp:   ctx.DocProperty,
			CheckType: "OptionalMiss",
		})
	}

	return issues
}

// FixRequiredness fixes the required/optional marker in a line
func (rc *RequirednesChecker) FixRequiredness(line, from, to string) string {
	if strings.Contains(line, from) {
		line = strings.Replace(line, from, to, 1)
	} else {
		// add after the first " - "
		if idx := strings.Index(line, " - "); idx > 0 {
			line = line[:idx+3] + to + " " + line[idx+3:]
		} else {
			// no dash, add after second backtick
			idx := strings.Index(line, "`")
			if idx >= 0 {
				secondIdx := strings.Index(line[idx+1:], "`")
				if secondIdx >= 0 {
					idx = idx + 1 + secondIdx + 1
					line = line[:idx] + " " + to + line[idx:]
				}
			}
		}
	}
	return line
}
