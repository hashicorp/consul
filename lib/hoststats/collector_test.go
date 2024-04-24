// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hoststats

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

// TestCollector_collect validates that metrics for host resource usage
// are collected as expected.
func TestCollector_collect(t *testing.T) {
	testcases := map[string]struct {
		skipDataDir bool
	}{
		"WithDataDirectory": {},
		"NoDataDirectory": {
			skipDataDir: true,
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			dataDir := ""
			if !tc.skipDataDir {
				dataDir = testutil.TempDir(t, "consul-config")
			}

			collector := initCollector(hclog.NewNullLogger(), dataDir)
			collector.collect()

			hs := collector.hostStats
			require.NotNil(t, hs)
			require.Greater(t, hs.Uptime, uint64(0))
			require.NotNil(t, hs.Memory)
			require.NotNil(t, hs.CPU)
			if !tc.skipDataDir {
				require.NotNil(t, hs.DataDirStats)
			} else {
				require.Nil(t, hs.DataDirStats)
			}
		})
	}
}
