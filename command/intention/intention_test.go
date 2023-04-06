// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package intention

import (
	"strings"
	"testing"
)

func TestCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
