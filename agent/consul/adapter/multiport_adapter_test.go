// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package adapter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestPopulateLegacyNodeServicePort_PreservesExplicitPort(t *testing.T) {
	service := &structs.NodeService{
		Port: 7000,
		Ports: structs.ServicePorts{
			{Name: "http", Port: 8080, Default: true},
		},
	}

	PopulateLegacyNodeServicePort(service)
	require.Equal(t, 7000, service.Port)
}
