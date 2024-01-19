// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package dns

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
)

// ParseLocality can parse peer name or datacenter from a DNS query's labels.
// Peer name is parsed from the same query part that datacenter is, so given this ambiguity
// we parse a "peerOrDatacenter". The caller or RPC handler are responsible for disambiguating.
func ParseLocality(labels []string, defaultEnterpriseMeta acl.EnterpriseMeta, _ enterpriseDNSConfig) (queryLocality, bool) {
	locality := queryLocality{
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
				locality.datacenter = labels[i]
			case "peer":
				locality.peer = labels[i]
			default:
				return queryLocality{}, false
			}
		}
		// Return error when both datacenter and peer are specified.
		if locality.datacenter != "" && locality.peer != "" {
			return queryLocality{}, false
		}
		return locality, true
	case 1:
		return queryLocality{peerOrDatacenter: labels[0]}, true

	case 0:
		return queryLocality{}, true
	}

	return queryLocality{}, false
}

// enterpriseDNSConfig is the configuration for enterprise DNS.
type enterpriseDNSConfig struct{}

// getEnterpriseDNSConfig returns the enterprise DNS configuration.
func getEnterpriseDNSConfig(conf *config.RuntimeConfig) enterpriseDNSConfig {
	return enterpriseDNSConfig{}
}
