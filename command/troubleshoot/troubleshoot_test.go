// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package troubleshoot

import (
	"strings"
	"testing"
)

func TestTroubleshootCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
