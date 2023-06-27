// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package local

import (
	"os"

	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/token"
)

// TestState returns a configured *State for testing.
func TestState(_ testing.T) *State {
	logger := hclog.New(&hclog.LoggerOptions{
		Output: os.Stderr,
	})

	result := NewState(Config{}, logger, &token.Store{})
	result.TriggerSyncChanges = func() {}
	return result
}
