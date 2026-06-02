// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package proxycfg

import "github.com/hashicorp/consul/agent/structs"

func (c *configSnapshotMeshGateway) hasEntExportedService(_ structs.ServiceName) bool {
	return false
}

func (c *configSnapshotMeshGateway) hasEntPartitionExport(_ structs.ServiceName) bool {
	return false
}

func (c *configSnapshotMeshGateway) entEmptyPeering() bool {
	return true
}
