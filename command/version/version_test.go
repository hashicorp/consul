// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package version

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestVersionCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}
