// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
)

// ParseLocality can parse peer name or datacenter from a DNS query's labels.
// Peer name is parsed from the same query part that datacenter is, so given this ambiguity
// we parse a "peerOrDatacenter". The caller or RPC handler are responsible for disambiguating.
func ParseLocality(labels []string, defaultEnterpriseMeta acl.EnterpriseMeta, _ EnterpriseDNSConfig) (QueryLocality, bool) {
	locality := QueryLocality{
		EnterpriseMeta: defaultEnterpriseMeta,
	}

	switch len(labels) {
	case 2, 4:
		// Support the following formats:
		// - [.<datacenter>.dc]
		// - [.<peer>.peer]
		for i := 0; i < len(labels); i += 2 {
			switch labels[i+1] {
			case "dc":
				locality.Datacenter = labels[i]
			case "peer":
				locality.Peer = labels[i]
			default:
				return QueryLocality{}, false
			}
		}
		// Return error when both datacenter and peer are specified.
		if locality.Datacenter != "" && locality.Peer != "" {
			return QueryLocality{}, false
		}
		return locality, true
	case 1:
		return QueryLocality{PeerOrDatacenter: labels[0]}, true

	case 0:
		return QueryLocality{}, true
	}

	return QueryLocality{}, false
}

// EnterpriseDNSConfig is the configuration for enterprise DNS.
type EnterpriseDNSConfig struct{}

// GetEnterpriseDNSConfig returns the enterprise DNS configuration.
func GetEnterpriseDNSConfig(conf *config.RuntimeConfig) EnterpriseDNSConfig {
	return EnterpriseDNSConfig{}
}
