// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package keygen

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
)

func TestKeygenCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKeygenCommand(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	cmd := New(ui)
	code := cmd.Run(nil)
	if code != 0 {
		t.Fatalf("bad: %d", code)
	}

	output := ui.OutputWriter.String()
	result, err := base64.StdEncoding.DecodeString(output)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if len(result) != 32 {
		t.Fatalf("bad: %#v", result)
	}
}
