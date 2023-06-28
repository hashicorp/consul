// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tls

import (
	"strings"
	"testing"
)

func TestValidateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
