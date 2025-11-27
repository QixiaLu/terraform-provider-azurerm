// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rule

import (
	"fmt"
	"regexp"
	"strings"
)

var ForceNewReg = regexp.MustCompile(` ?Changing.*forces? a [^.]*(\.|$)`)

// ForceNewChecker performs ForceNew marker consistency checks
type ForceNewChecker struct{}

// CheckMarkers validates that ForceNew markers in documentation match schema
// Skips resource_group_name field as it has special handling
// Returns issues if ForceNew markers are inconsistent
func (fc *ForceNewChecker) CheckMarkers(ctx *PropertyCheckContext) []*CheckIssue {
	var issues []*CheckIssue

	if ctx.DocProperty == nil || ctx.SchemaProperty == nil {
		return issues
	}

	// Skip resource_group_name as per existing logic
	if lastPathSegment(ctx.FullPath) == "resource_group_name" {
		return issues
	}

	// Check: ForceNew markers
	if ctx.SchemaProperty.ForceNew != ctx.DocProperty.ForceNew {
		if ctx.SchemaProperty.ForceNew && !ctx.DocProperty.ForceNew {
			issues = append(issues, &CheckIssue{
				LineNum:   ctx.DocProperty.Line,
				Key:       ctx.FullPath,
				Message:   fmt.Sprintf("S006: `%s` should be marked as ForceNew", ctx.FullPath),
				FixLine:   fc.FixForceNew(ctx.DocProperty.Content, true),
				Line:      ctx.DocProperty.Content,
				DocProp:   ctx.DocProperty,
				CheckType: "ForceNewMiss",
			})
		} else if ctx.DocProperty.ForceNew && !ctx.SchemaProperty.ForceNew {
			issues = append(issues, &CheckIssue{
				LineNum:   ctx.DocProperty.Line,
				Key:       ctx.FullPath,
				Message:   fmt.Sprintf("S006: `%s` should not be marked as ForceNew", ctx.FullPath),
				FixLine:   fc.FixForceNew(ctx.DocProperty.Content, false),
				Line:      ctx.DocProperty.Content,
				DocProp:   ctx.DocProperty,
				CheckType: "ForceNewMiss",
			})
		}
	}

	return issues
}

// FixForceNew fixes the ForceNew marker in a line
func (fc *ForceNewChecker) FixForceNew(line string, shouldAdd bool) string {
	if shouldAdd {
		// Add ForceNew message if not present
		line = strings.TrimRight(line, " ")
		if strings.HasSuffix(line, ",") {
			line = line[:len(line)-1] + "."
		} else if !strings.HasSuffix(line, ".") {
			line += "."
		}
		line += " Changing this forces a new resource to be created."
	} else {
		line = ForceNewReg.ReplaceAllString(line, "")
	}
	return line
}

// lastPathSegment extracts the last segment of a dotted path
func lastPathSegment(path string) string {
	if idx := strings.LastIndex(path, "."); idx >= 0 {
		return path[idx+1:]
	}
	return path
}
