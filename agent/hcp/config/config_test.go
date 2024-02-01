// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package config

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMerge(t *testing.T) {
	oldCfg := CloudConfig{
		ResourceID:      "old-resource-id",
		ClientID:        "old-client-id",
		ClientSecret:    "old-client-secret",
		Hostname:        "old-hostname",
		AuthURL:         "old-auth-url",
		ScadaAddress:    "old-scada-address",
		ManagementToken: "old-token",
		TLSConfig: &tls.Config{
			ServerName: "old-server-name",
		},
		NodeID:   "old-node-id",
		NodeName: "old-node-name",
	}

	newCfg := CloudConfig{
		ResourceID:      "new-resource-id",
		ClientID:        "new-client-id",
		ClientSecret:    "new-client-secret",
		Hostname:        "new-hostname",
		AuthURL:         "new-auth-url",
		ScadaAddress:    "new-scada-address",
		ManagementToken: "new-token",
		TLSConfig: &tls.Config{
			ServerName: "new-server-name",
		},
		NodeID:   "new-node-id",
		NodeName: "new-node-name",
	}

	for name, tc := range map[string]struct {
		newCfg      CloudConfig
		expectedCfg CloudConfig
	}{
		"Empty": {
			newCfg:      CloudConfig{},
			expectedCfg: oldCfg,
		},
		"All": {
			newCfg:      newCfg,
			expectedCfg: newCfg,
		},
		"Partial": {
			newCfg: CloudConfig{
				ResourceID:      newCfg.ResourceID,
				ClientID:        newCfg.ClientID,
				ClientSecret:    newCfg.ClientSecret,
				ManagementToken: newCfg.ManagementToken,
			},
			expectedCfg: CloudConfig{
				ResourceID:      newCfg.ResourceID,
				ClientID:        newCfg.ClientID,
				ClientSecret:    newCfg.ClientSecret,
				ManagementToken: newCfg.ManagementToken,
				Hostname:        oldCfg.Hostname,
				AuthURL:         oldCfg.AuthURL,
				ScadaAddress:    oldCfg.ScadaAddress,
				TLSConfig:       oldCfg.TLSConfig,
				NodeID:          oldCfg.NodeID,
				NodeName:        oldCfg.NodeName,
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			merged := Merge(oldCfg, tc.newCfg)
			require.Equal(t, tc.expectedCfg, merged)
		})
	}
}
