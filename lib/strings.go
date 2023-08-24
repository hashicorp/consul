// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package lib

import (
	"strings"
)

// EnsureTrailingNewline adds a newline suffix to the input if not present.
// This is typically used to fix a case where the CA provider does not return a new line
// after certificates as per the specification. See GH-8178 for more context.
func EnsureTrailingNewline(str string) string {
	if str == "" {
		return str
	}
	if strings.HasSuffix(str, "\n") {
		return str
	}
	return str + "\n"
}
