// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package operator

import (
	"strings"
	"testing"
)

func TestOperatorCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New().Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
