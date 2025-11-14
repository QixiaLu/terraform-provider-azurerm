// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

// PositionType represents where a field appears in documentation
type PositionType int

const (
	PosDefault PositionType = iota
	PosExample
	PosArgs
	PosAttr
	PosTimeout
	PosImport
	PosOther = 100
)

func (p PositionType) String() string {
	names := [...]string{
		"Default",
		"Example",
		"Args",
		"Attr",
		"Timeout",
		"Import",
	}
	if p == PosOther {
		return "Other"
	}
	if int(p) < len(names) {
		return names[p]
	}
	return "Unknown"
}

func (p PositionType) IsArgOrAttr() bool {
	return p == PosArgs || p == PosAttr
}

// RequiredType represents field requirement status
type RequiredType int

const (
	RequiredDefault  RequiredType = 0
	RequiredOptional RequiredType = 1 << iota
	RequiredRequired
	RequiredComputed
)

func (r RequiredType) String() string {
	switch r {
	case RequiredDefault:
		return "Default"
	case RequiredOptional:
		return "Optional"
	case RequiredRequired:
		return "Required"
	case RequiredComputed:
		return "Computed"
	default:
		return "Unknown"
	}
}

func (r RequiredType) IsRequired() bool {
	return r&RequiredRequired != 0
}

func (r RequiredType) IsOptional() bool {
	return r&RequiredOptional != 0
}

func (r RequiredType) IsComputed() bool {
	return r&RequiredComputed != 0
}
