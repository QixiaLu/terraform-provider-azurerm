// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package markdown

import (
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-provider-azurerm/internal/tools/document-fmt/parser"
)

type AttributesSection struct {
	heading      Heading
	content      []string
	parsedFields *parser.ParsedProperties // cached parsed fields
}

var _ SectionWithTemplate = &AttributesSection{}

func (s *AttributesSection) Match(line string) bool {
	return regexp.MustCompile(`#+(\s)*attribute.*`).MatchString(strings.ToLower(line))
}

func (s *AttributesSection) SetHeading(line string) {
	s.heading = NewHeading(line)
}

func (s *AttributesSection) GetHeading() Heading {
	return s.heading
}

func (s *AttributesSection) SetContent(content []string) {
	s.content = content
	// Clear cached parsed fields when content changes
	s.parsedFields = nil
}

func (s *AttributesSection) GetContent() []string {
	return s.content
}

func (s *AttributesSection) Template() string {
	// TODO implement me
	panic("implement me")
}
