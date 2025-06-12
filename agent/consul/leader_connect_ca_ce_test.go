// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestValidateSupportedIdentityScopesForServiceInCertificate(t *testing.T) {
	config := DefaultConfig()
	manager := NewCAManager(nil, nil, testutil.Logger(t), config)

	tests := []struct {
		name       string
		expectErr  string
		datacenter string
		namespace  string
		service    string
		partition  string
	}{
		{
			name: "err_unsupported_partition_and_namespace_for_service",
			expectErr: "Non default partition or namespace is supported in Enterprise only." +
				"Provided namespace is test-namespace and partition is test-partition",
			namespace: "test-namespace",
			service:   "test-service",
			partition: "test-partition",
		},
		{
			name: "err_unsupported_namespace_for_service",
			expectErr: "Non default partition or namespace is supported in Enterprise only." +
				"Provided namespace is test-namespace and partition is default",
			namespace: "test-namespace",
			service:   "test-service",
			partition: "default",
		},
		{
			name: "err_unsupported_partition_for_service",
			expectErr: "Non default partition or namespace is supported in Enterprise only." +
				"Provided namespace is default and partition is test-partition",
			namespace: "default",
			service:   "test-service",
			partition: "test-partition",
		},
		{
			name:      "default_partition_and_namespace_supported_for_service",
			expectErr: "",
			namespace: "default",
			service:   "test-service",
			partition: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spiffeIDService := &connect.SpiffeIDService{
				Host:       config.NodeName,
				Datacenter: tc.datacenter,
				Namespace:  tc.namespace,
				Service:    tc.service,
				Partition:  tc.partition,
			}
			err := manager.validateSupportedIdentityScopesInCertificate(spiffeIDService)
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSupportedIdentityScopesForMeshGatewayInCertificate(t *testing.T) {
	config := DefaultConfig()
	manager := NewCAManager(nil, nil, testutil.Logger(t), config)

	tests := []struct {
		name      string
		expectErr string
		partition string
	}{
		{
			name: "err_unsupported_partition_for_mesh_gateway",
			expectErr: "Non default partition is supported in Enterprise only." +
				"Provided partition is test-partition",
			partition: "test-partition",
		},
		{
			name:      "default_partition_supported_for_mesh_gateway",
			expectErr: "",
			partition: "default",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spiffeIDMeshGateway := &connect.SpiffeIDMeshGateway{
				Host:       config.NodeName,
				Datacenter: config.Datacenter,
				Partition:  tc.partition,
			}
			err := manager.validateSupportedIdentityScopesInCertificate(spiffeIDMeshGateway)
			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateSupportedIdentityScopesForServerInCertificate(t *testing.T) {
	config := DefaultConfig()
	manager := NewCAManager(nil, nil, testutil.Logger(t), config)
	spiffeIDServer := &connect.SpiffeIDServer{
		Host: config.NodeName, Datacenter: config.Datacenter}
	err := manager.validateSupportedIdentityScopesInCertificate(spiffeIDServer)
	require.NoError(t, err)
}

func TestValidateSupportedIdentityScopesForAgentInCertificate(t *testing.T) {
	config := DefaultConfig()
	manager := NewCAManager(nil, nil, testutil.Logger(t), config)
	spiffeIDAgent := &connect.SpiffeIDAgent{
		Host: config.NodeName, Datacenter: config.Datacenter, Partition: "test-partition", Agent: "test-agent"}
	err := manager.validateSupportedIdentityScopesInCertificate(spiffeIDAgent)
	require.NoError(t, err)
}
