//go:build tools
// +build tools

// Package tools tracks tool dependencies for the project.
// These are not imported in the actual code but we want go.mod to track them.
package tools

import (
	_ "github.com/evilmartians/lefthook"
)
