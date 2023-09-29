//go:build !consulent
// +build !consulent

package token

import (
	"testing"
)

func TestFormatTokenExpanded(t *testing.T) {
	testFormatTokenExpanded(t, "FormatTokenExpanded/ce")
}
