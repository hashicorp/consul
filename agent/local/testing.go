// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package local

import (
	"os"

	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/go-testing-interface"
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
