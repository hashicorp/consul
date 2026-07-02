// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package query

import (
	"strings"
	"testing"
)

func TestQueryCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
