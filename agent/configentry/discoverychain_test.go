// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package configentry

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

// TestDiscoveryChainSetHashConsistency ensures hashing is deterministic.
func TestDiscoveryChainSetHashConsistency(t *testing.T) {
	dcs := NewDiscoveryChainSet()
	dcs.AddRouters(&structs.ServiceRouterConfigEntry{
		Kind: structs.ServiceRouter,
		Name: "router1",
	})
	dcs.AddSplitters(&structs.ServiceSplitterConfigEntry{
		Kind: structs.ServiceSplitter,
		Name: "splitter1",
	})

	hash1 := dcs.Hash()
	hash2 := dcs.Hash()
	hash3 := dcs.Hash()

	if hash1 != hash2 || hash2 != hash3 {
		t.Errorf("Hash() is not consistent. Got: %d, %d, %d", hash1, hash2, hash3)
	}
}
