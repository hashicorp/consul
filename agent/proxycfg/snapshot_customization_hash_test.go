// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// TestAlignSynthesizedCustomizationHashes pins the fix for the api-gateway
// route/cluster name mismatch.
//
// Envoy cluster names are CustomizeClusterName(target.Name, chain), which
// prefixes "<hash>~" when the chain has a CustomizationHash. Clusters are
// generated from the per-service chain (compiled with the listener protocol as
// OverrideProtocol, so it carries a hash whenever the listener protocol differs
// from the backend's own protocol), while routes are generated from the
// synthesized route chain (compiled with no override, so it has no hash). Left
// unaligned the route names a cluster that does not exist and Envoy 503s every
// request with http.ingress_upstream_<port>.no_cluster.
func TestAlignSynthesizedCustomizationHashes(t *testing.T) {
	serviceChain := func(name, hash string) *structs.CompiledDiscoveryChain {
		return &structs.CompiledDiscoveryChain{
			ServiceName:       name,
			CustomizationHash: hash,
		}
	}
	upstreamID := func(name string) UpstreamID {
		em := acl.NewEnterpriseMetaWithPartition("default", "default")
		return NewUpstreamIDFromServiceName(structs.NewServiceName(name, &em))
	}
	target := func(service string) *structs.DiscoveryTarget {
		return &structs.DiscoveryTarget{
			ID:        service,
			Service:   service,
			Namespace: "default",
			Partition: "default",
		}
	}
	routeChain := func(name string, targets ...*structs.DiscoveryTarget) *structs.CompiledDiscoveryChain {
		m := map[string]*structs.DiscoveryTarget{}
		for _, tgt := range targets {
			m[tgt.ID] = tgt
		}
		return &structs.CompiledDiscoveryChain{ServiceName: name, Targets: m}
	}

	t.Run("adopts the hash of a customized backend", func(t *testing.T) {
		// grpc listener in front of an http backend: the backend chain is
		// customized, so the route must use the same prefixed cluster name.
		chain := routeChain("route-one", target("backend"))
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{
				upstreamID("backend"): serviceChain("backend", "ef15b5b5"),
			},
		)
		require.Empty(t, conflicts)
		require.Equal(t, "ef15b5b5", chain.CustomizationHash)
	})

	t.Run("leaves aligned backends unprefixed", func(t *testing.T) {
		// Listener protocol matches the backend protocol, so no customization
		// exists on either side and the plain name is already correct.
		chain := routeChain("route-two", target("backend"))
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{
				upstreamID("backend"): serviceChain("backend", ""),
			},
		)
		require.Empty(t, conflicts)
		require.Equal(t, "", chain.CustomizationHash)
	})

	t.Run("reports rather than guesses when backends disagree", func(t *testing.T) {
		// A single chain carries a single hash applied to every target, so a
		// route fanning out to differently-customized backends cannot be
		// expressed. Stamping either hash would replace a missing cluster with
		// a wrong one, so the chain is left untouched and reported.
		chain := routeChain("route-three", target("customized"), target("plain"))
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{
				upstreamID("customized"): serviceChain("customized", "ef15b5b5"),
				upstreamID("plain"):      serviceChain("plain", ""),
			},
		)
		require.Len(t, conflicts, 1)
		require.Contains(t, conflicts[0].Error(), "route-three")
		require.Equal(t, "", chain.CustomizationHash)
	})

	t.Run("never overwrites a hash the chain already has", func(t *testing.T) {
		chain := routeChain("route-four", target("backend"))
		chain.CustomizationHash = "original"
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{
				upstreamID("backend"): serviceChain("backend", "different"),
			},
		)
		require.Empty(t, conflicts)
		require.Equal(t, "original", chain.CustomizationHash)
	})

	t.Run("ignores external targets and unknown services", func(t *testing.T) {
		ext := target("external")
		ext.External = true
		chain := routeChain("route-five", ext, target("not-in-snapshot"))
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{},
		)
		require.Empty(t, conflicts)
		require.Equal(t, "", chain.CustomizationHash)
	})

	t.Run("peered backends are aligned", func(t *testing.T) {
		// A peered backend is reached through a service-resolver redirect, so
		// the target carries Peer while the chain is still keyed by the local
		// service name. It is a normal backend for naming purposes: the cluster
		// path emits the hashed name, so the route must use it too. An
		// api-gateway grpc/http2 listener in front of a peered service 503s
		// every request otherwise.
		peer := target("backend")
		peer.Peer = "paymentpeer"
		chain := routeChain("route-peered", peer)
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{
				upstreamID("backend"): serviceChain("backend", "c225dc1c"),
			},
		)
		require.Empty(t, conflicts)
		require.Equal(t, "c225dc1c", chain.CustomizationHash)
	})

	t.Run("ignores a chain's reference to itself", func(t *testing.T) {
		// A synthesized api-gateway chain is named for its listener, is itself
		// published into serviceChains, and lists that same name among its
		// targets. Counting that self-reference reads back the hash this call
		// has not set yet (""), which collides with the real backend hash and
		// sends an otherwise-unanimous chain down the disagreement path. That
		// is what left peered api-gateway routes pointing at a cluster that
		// does not exist under enterprise builds.
		self := target("route-self")
		chain := routeChain("route-self", self, target("backend"))
		conflicts := alignSynthesizedCustomizationHashes(
			[]*structs.CompiledDiscoveryChain{chain},
			map[UpstreamID]*structs.CompiledDiscoveryChain{
				upstreamID("route-self"): chain,
				upstreamID("backend"):    serviceChain("backend", "ef15b5b5"),
			},
		)
		require.Empty(t, conflicts,
			"the self-reference must not count as a disagreeing backend")
		require.Equal(t, "ef15b5b5", chain.CustomizationHash)
	})

	t.Run("tolerates nil chains", func(t *testing.T) {
		require.NotPanics(t, func() {
			alignSynthesizedCustomizationHashes(
				[]*structs.CompiledDiscoveryChain{nil},
				map[UpstreamID]*structs.CompiledDiscoveryChain{},
			)
		})
	})
}
