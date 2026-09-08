// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build consulent

package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

// TestGenerateAPIGatewayDNSSANs_Partition verifies that on Enterprise the API
// gateway leaf-cert DNS SANs carry a partition-scoped wildcard following the
// tagged-subdomain grammar "*.api-gateway.<namespace>.<partition>.<domain>"
// (partition is the broader scope, so it sits closer to the domain). This is
// the TLS-verification half of the tenancy handling required by the
// API-Gateway DNS auto-registration RFC; the namespace-only behavior is covered
// by TestGenerateAPIGatewayDNSSANs in the CE-buildable test file.
func TestGenerateAPIGatewayDNSSANs_Partition(t *testing.T) {
	h := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				source:    &structs.QuerySource{Datacenter: "dc1"},
				dnsConfig: DNSConfig{Domain: "consul", AltDomain: "alt.consul"},
			},
		},
	}

	snap := &ConfigSnapshot{}
	snap.APIGateway.Upstreams = listenerRouteUpstreams{}

	route := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "r1"}
	listenerKey := APIGatewayListenerKey{Protocol: "http", Port: 8080}
	snap.APIGateway.Upstreams.set(route, listenerKey, structs.Upstreams{
		{
			DestinationName:      "web",
			DestinationNamespace: "payments",
			DestinationPartition: "team-a",
		},
	})

	sans := h.generateAPIGatewayDNSSANs(snap)

	// Partition-scoped wildcards (namespace nearer the label, partition nearer
	// the domain), for both the primary and alt DNS domains and their
	// datacenter-qualified forms.
	require.Contains(t, sans, "*.api-gateway.payments.team-a.consul")
	require.Contains(t, sans, "*.api-gateway.payments.team-a.dc1.consul")
	require.Contains(t, sans, "*.api-gateway.payments.team-a.alt.consul")
	require.Contains(t, sans, "*.api-gateway.payments.team-a.dc1.alt.consul")
}

// TestGenerateAPIGatewayDNSSANs_DefaultPartition verifies that an upstream in a
// non-default namespace but the default partition omits the partition segment,
// so the emitted SAN matches the namespace-only form (identical to CE output).
func TestGenerateAPIGatewayDNSSANs_DefaultPartition(t *testing.T) {
	h := &handlerAPIGateway{
		handlerState: handlerState{
			stateConfig: stateConfig{
				source:    &structs.QuerySource{Datacenter: "dc1"},
				dnsConfig: DNSConfig{Domain: "consul"},
			},
		},
	}

	snap := &ConfigSnapshot{}
	snap.APIGateway.Upstreams = listenerRouteUpstreams{}

	route := structs.ResourceReference{Kind: structs.HTTPRoute, Name: "r1"}
	listenerKey := APIGatewayListenerKey{Protocol: "http", Port: 8080}
	snap.APIGateway.Upstreams.set(route, listenerKey, structs.Upstreams{
		{
			DestinationName:      "web",
			DestinationNamespace: "payments",
			DestinationPartition: "default",
		},
	})

	sans := h.generateAPIGatewayDNSSANs(snap)

	require.Contains(t, sans, "*.api-gateway.payments.consul")
	require.Contains(t, sans, "*.api-gateway.payments.dc1.consul")
	require.NotContains(t, sans, "*.api-gateway.payments.default.consul")
}
