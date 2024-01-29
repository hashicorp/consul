// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discovery

import "github.com/hashicorp/consul/acl"

// QueryLocality is the locality parsed from a DNS query.
type QueryLocality struct {
	// Datacenter is the datacenter parsed from a label that has an explicit datacenter part.
	// Example query: <service>.virtual.<namespace>.ns.<partition>.ap.<datacenter>.dc.consul
	Datacenter string

	// Peer is the peer name parsed from a label that has explicit parts.
	// Example query: <service>.virtual.<namespace>.ns.<peer>.peer.<partition>.ap.consul
	Peer string

	// PeerOrDatacenter is parsed from DNS queries where the datacenter and peer name are
	// specified in the same query part.
	// Example query: <service>.virtual.<peerOrDatacenter>.consul
	//
	// Note that this field should only be a "peer" for virtual queries, since virtual IPs should
	// not be shared between datacenters. In all other cases, it should be considered a DC.
	PeerOrDatacenter string

	acl.EnterpriseMeta
}

// EffectiveDatacenter returns the datacenter parsed from a query, or a default
// value if none is specified.
func (l QueryLocality) EffectiveDatacenter(defaultDC string) string {
	// Prefer the value parsed from a query with explicit parts: <namespace>.ns.<partition>.ap.<datacenter>.dc
	if l.Datacenter != "" {
		return l.Datacenter
	}
	// Fall back to the ambiguously parsed DC or Peer.
	if l.PeerOrDatacenter != "" {
		return l.PeerOrDatacenter
	}
	// If all are empty, use a default value.
	return defaultDC
}

// GetQueryTenancyBasedOnLocality returns a discovery.QueryTenancy from a DNS message.
func GetQueryTenancyBasedOnLocality(locality QueryLocality, defaultDatacenter string) (QueryTenancy, error) {
	datacenter := locality.EffectiveDatacenter(defaultDatacenter)
	// Only one of dc or peer can be used.
	if locality.Peer != "" {
		datacenter = ""
	}

	return QueryTenancy{
		EnterpriseMeta: locality.EnterpriseMeta,
		// The datacenter of the request is not specified because cross-datacenter virtual IP
		// queries are not supported. This guard rail is in place because virtual IPs are allocated
		// within a DC, therefore their uniqueness is not guaranteed globally.
		Peer:          locality.Peer,
		Datacenter:    datacenter,
		SamenessGroup: "", // this should be nil since the single locality was directly used to configure tenancy.
	}, nil
}
