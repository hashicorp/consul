// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package agent

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	agentdns "github.com/hashicorp/consul/agent/dns"
	"github.com/hashicorp/consul/agent/structs"
)

// NOTE: these functions have also been copied to agent/dns package for dns v2.
// If you change these functions, please also change the ones in agent/dns as well.
// These v1 versions will soon be deprecated.

type enterpriseDNSConfig struct{}

func getEnterpriseDNSConfig(conf *config.RuntimeConfig) enterpriseDNSConfig {
	return enterpriseDNSConfig{}
}

// parseLocality can parse peer name or datacenter from a DNS query's labels.
// Peer name is parsed from the same query part that datacenter is, so given this ambiguity
// we parse a "peerOrDatacenter". The caller or RPC handler are responsible for disambiguating.
func (d *DNSServer) parseLocality(labels []string, cfg *dnsRequestConfig) (queryLocality, bool) {
	locality := queryLocality{
		EnterpriseMeta: cfg.defaultEnterpriseMeta,
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
		return queryLocality{
			peerOrDatacenter: labels[0],
			EnterpriseMeta:   cfg.defaultEnterpriseMeta,
		}, true

	case 0:
		return queryLocality{}, true
	}

	return queryLocality{}, false
}

type querySameness struct{}

// parseSamenessGroupLocality wraps parseLocality in CE
func (d *DNSServer) parseSamenessGroupLocality(cfg *dnsRequestConfig, labels []string, errfnc func() error) (queryLocality, error) {
	locality, ok := d.parseLocality(labels, cfg)
	if !ok {
		return queryLocality{
			EnterpriseMeta: cfg.defaultEnterpriseMeta,
		}, errfnc()
	}
	return locality, nil
}

func serviceCanonicalDNSName(name, kind, datacenter, domain string, _ *acl.EnterpriseMeta) string {
	return fmt.Sprintf("%s.%s.%s.%s", name, kind, datacenter, domain)
}

func nodeCanonicalDNSName(node *structs.Node, respDomain string) string {
	if node.PeerName != "" {
		// We must return a more-specific DNS name for peering so
		// that there is no ambiguity with lookups.
		return fmt.Sprintf("%s.node.%s.peer.%s",
			node.Node,
			node.PeerName,
			respDomain)
	}
	// Return a simpler format for non-peering nodes.
	return fmt.Sprintf("%s.node.%s.%s", node.Node, node.Datacenter, respDomain)
}

// setEnterpriseMetaFromRequestContext sets the DefaultNamespace and DefaultPartition on the requestDnsConfig
// based on the requestContext's DefaultNamespace and DefaultPartition.
func (d *DNSServer) setEnterpriseMetaFromRequestContext(requestContext agentdns.Context, requestDnsConfig *dnsRequestConfig) {
	// do nothing
}
