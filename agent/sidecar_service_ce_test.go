// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestSidecarServiceFromNodeService_DoesNotCopyPortsInCE(t *testing.T) {
	t.Parallel()

	ns := (&structs.ServiceDefinition{
		ID:   "web1",
		Name: "web",
		Ports: structs.ServicePorts{
			{Name: "http", Port: 8080, Default: true},
			{Name: "metrics", Port: 9090},
		},
		Connect: &structs.ServiceConnect{
			SidecarService: &structs.ServiceDefinition{},
		},
	}).NodeService()

	sidecar, _, _, err := sidecarServiceFromNodeService(ns, "")
	require.NoError(t, err)
	require.NotNil(t, sidecar)
	require.Empty(t, sidecar.Ports)
	require.Empty(t, sidecar.Proxy.LocalServicePorts)
	require.Equal(t, 8080, sidecar.Proxy.LocalServicePort)
}
