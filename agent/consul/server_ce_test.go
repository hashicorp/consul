// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package consul

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/testrpc"
)

func TestAgent_ReloadConfig_Reporting(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()

	dir1, s := testServerWithConfig(t, func(c *Config) {
		c.Reporting.License.Enabled = false
	})
	defer os.RemoveAll(dir1)
	defer s.Shutdown()

	testrpc.WaitForTestAgent(t, s.RPC, "dc1")

	require.Equal(t, false, s.config.Reporting.License.Enabled)

	rc := ReloadableConfig{
		Reporting: Reporting{
			License: License{
				Enabled: true,
			},
		},
	}

	require.NoError(t, s.ReloadConfig(rc))

	// Check config reload is no-op
	require.Equal(t, false, s.config.Reporting.License.Enabled)
}
