// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package scada

import (
	"testing"

	"github.com/hashicorp/consul/agent/hcp/config"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

func TestUpdateHCPConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg         config.CloudConfig
		expectedErr string
	}{
		"Success": {
			cfg: config.CloudConfig{
				ResourceID:   "organization/85702e73-8a3d-47dc-291c-379b783c5804/project/8c0547c0-10e8-1ea2-dffe-384bee8da634/hashicorp.consul.global-network-manager.cluster/test",
				ClientID:     "test",
				ClientSecret: "test",
			},
		},
		"Empty": {
			cfg:         config.CloudConfig{},
			expectedErr: "could not parse resource: unexpected number of tokens 1",
		},
		"InvalidResource": {
			cfg: config.CloudConfig{
				ResourceID: "invalid",
			},
			expectedErr: "could not parse resource: unexpected number of tokens 1",
		},
	} {
		t.Run(name, func(t *testing.T) {
			// Create a provider
			p, err := New(hclog.NewNullLogger())
			require.NoError(t, err)

			// Update the provider
			err = p.UpdateHCPConfig(tc.cfg)
			if tc.expectedErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
				return
			}
			require.NoError(t, err)
		})
	}
}
