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

	// WatchedModels tracks the per-candidate health watches so they can be
	// cancelled when a candidate is removed from the routing policy.
	WatchedModels map[structs.ServiceName]context.CancelFunc

	// Models holds the discovered model upstreams keyed by service name. Only
	// services whose instances carry ai.role == "ai-model" are kept; the model's
	// catalog Meta (labels) and healthy endpoints are recorded for routing and
	// for injection into the gateway listener metadata.
	Models map[structs.ServiceName]*InferenceGatewayModel
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
		len(c.WatchedModels) == 0 &&
		len(c.Models) == 0
}
