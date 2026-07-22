// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"

	"github.com/hashicorp/consul/agent/structs"
)

// configSnapshotInferenceGateway holds the proxycfg state for an inference
// gateway: its own inbound-mTLS identity, the bound ai-gateway routing policy,
// and the set of discovered model upstreams (ai.role == "ai-model").
type configSnapshotInferenceGateway struct {
	// Leaf is the gateway's own leaf cert, used to terminate inbound mesh mTLS
	// from calling agents.
	Leaf *structs.IssuedCert

	// MeshConfig is the global mesh config entry.
	MeshConfig    *structs.MeshConfigEntry
	MeshConfigSet bool

	// GatewayConfig is the bound ai-gateway config entry (the routing policy).
	GatewayConfig    *structs.AIGatewayConfigEntry
	GatewayConfigSet bool

	// DiscoveredUpstreams is the set of services the gateway is intention-allowed
	// to reach, inferred from intentions (the same data source connect_proxy uses
	// for transparent-proxy discovery). This drives model discovery: the gateway
	// discovers exactly the services catalog ∩ intentions permits, and updateModel
	// then keeps only those tagged ai.role == "ai-model".
	DiscoveredUpstreams structs.ServiceList

	// WatchedModels tracks the per-candidate health watches so they can be
	// cancelled when a candidate is removed from the discovered set.
	WatchedModels map[structs.ServiceName]context.CancelFunc

	// Models holds the discovered model upstreams keyed by service name. Only
	// services whose instances carry ai.role == "ai-model" are kept; the model's
	// catalog Meta (labels) and healthy endpoints are recorded for routing and
	// for injection into the gateway listener metadata.
	Models map[structs.ServiceName]*InferenceGatewayModel

	// StateStore is the mesh service backing the rate-limit counter, named by the
	// bound ai-gateway entry's StateStore block. Unlike Models it is reached as a
	// Connect mesh upstream (mTLS, intention-gated) — NOT via the terminating
	// gateway and NOT ai.role-filtered — so it is watched and rendered separately.
	//
	// StateStoreService is the currently-watched store service (zero value = none,
	// which is also how a removed StateStore is detected); StateStoreCancel cancels
	// its Connect health watch; StateStoreNodes holds its healthy Connect endpoints
	// for the mTLS EDS cluster + load assignment.
	StateStoreService structs.ServiceName
	StateStoreCancel  context.CancelFunc
	StateStoreNodes   structs.CheckServiceNodes
}

// InferenceGatewayModel is a discovered model upstream.
type InferenceGatewayModel struct {
	Service structs.ServiceName
	Role    string
	Labels  map[string]string
	Nodes   structs.CheckServiceNodes
}

func (c *configSnapshotInferenceGateway) valid() bool {
	return c.GatewayConfigSet && c.MeshConfigSet
}

// isEmpty reports whether the snapshot has been initialized.
func (c *configSnapshotInferenceGateway) isEmpty() bool {
	if c == nil {
		return true
	}
	return c.Leaf == nil &&
		!c.MeshConfigSet &&
		!c.GatewayConfigSet &&
		len(c.DiscoveredUpstreams) == 0 &&
		len(c.WatchedModels) == 0 &&
		len(c.Models) == 0 &&
		c.StateStoreService.Name == "" &&
		len(c.StateStoreNodes) == 0
}
