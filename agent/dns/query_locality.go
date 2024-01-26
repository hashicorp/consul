// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import "github.com/hashicorp/consul/acl"

// queryLocality is the locality parsed from a DNS query.
type queryLocality struct {
	// datacenter is the datacenter parsed from a label that has an explicit datacenter part.
	// Example query: <service>.virtual.<namespace>.ns.<partition>.ap.<datacenter>.dc.consul
	datacenter string

	// peer is the peer name parsed from a label that has explicit parts.
	// Example query: <service>.virtual.<namespace>.ns.<peer>.peer.<partition>.ap.consul
	peer string

	// peerOrDatacenter is parsed from DNS queries where the datacenter and peer name are
	// specified in the same query part.
	// Example query: <service>.virtual.<peerOrDatacenter>.consul
	//
	// Note that this field should only be a "peer" for virtual queries, since virtual IPs should
	// not be shared between datacenters. In all other cases, it should be considered a DC.
	peerOrDatacenter string

	acl.EnterpriseMeta
}

// EffectiveDatacenter returns the datacenter parsed from a query, or a default
// value if none is specified.
func (l queryLocality) EffectiveDatacenter(defaultDC string) string {
	// Prefer the value parsed from a query with explicit parts: <namespace>.ns.<partition>.ap.<datacenter>.dc
	if l.datacenter != "" {
		return l.datacenter
	}
	// Fall back to the ambiguously parsed DC or Peer.
	if l.peerOrDatacenter != "" {
		return l.peerOrDatacenter
	}
	// If all are empty, use a default value.
	return defaultDC
}
