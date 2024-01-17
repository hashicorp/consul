// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"strings"

	"github.com/miekg/dns"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/discovery"
)

// buildQueryFromDNSMessage returns a discovery.Query from a DNS message.
func buildQueryFromDNSMessage(req *dns.Msg, domain, altDomain string, cfg *RouterDynamicConfig, defaultEntMeta acl.EnterpriseMeta) (*discovery.Query, error) {
	queryType, queryParts, querySuffixes := getQueryTypePartsAndSuffixesFromDNSMessage(req, domain, altDomain)

	locality, ok := ParseLocality(querySuffixes, defaultEntMeta, cfg.enterpriseDNSConfig)
	if !ok {
		return nil, errors.New("invalid locality")
	}

	// TODO(v2-dns): This needs to be deprecated.
	peerName := locality.peer
	if peerName == "" {
		// If the peer name was not explicitly defined, fall back to the ambiguously-parsed version.
		peerName = locality.peerOrDatacenter
	}

	return &discovery.Query{
		QueryType: queryType,
		QueryPayload: discovery.QueryPayload{
			Name: queryParts[len(queryParts)-1],
			Tenancy: discovery.QueryTenancy{
				EnterpriseMeta: locality.EnterpriseMeta,
				// v2-dns: revisit if we need this after the rest of this works.
				//	SamenessGroup: "",
				// The datacenter of the request is not specified because cross-datacenter virtual IP
				// queries are not supported. This guard rail is in place because virtual IPs are allocated
				// within a DC, therefore their uniqueness is not guaranteed globally.
				Peer:       peerName,
				Datacenter: locality.datacenter,
			},
			// TODO(v2-dns): what should these be?
			//PortName:   "",
			//Tag:        "",
			//RemoteAddr: nil,
			//DisableFailover: false,
		},
	}, nil
}

// getQueryTypePartsAndSuffixesFromDNSMessage returns the query type, the parts, and suffixes of the query name.
func getQueryTypePartsAndSuffixesFromDNSMessage(req *dns.Msg, domain, altDomain string) (queryType discovery.QueryType, parts []string, suffixes []string) {
	// Get the QName without the domain suffix
	qName := trimDomainFromQuestionName(req.Question[0].Name, domain, altDomain)

	// Split into the label parts
	labels := dns.SplitDomainName(qName)

	done := false
	for i := len(labels) - 1; i >= 0 && !done; i-- {
		queryType = getQueryTypeFromLabels(labels[i])
		switch queryType {
		case discovery.QueryTypeInvalid:
			// If we don't recognize the query type, we keep going until we find one we do.
		case discovery.QueryTypeService,
			discovery.QueryTypeConnect, discovery.QueryTypeVirtual, discovery.QueryTypeIngress,
			discovery.QueryTypeNode, discovery.QueryTypePreparedQuery:
			parts = labels[:i]
			suffixes = labels[i+1:]
			done = true
		default:
			// If this is a SRV query the "service" label is optional, we add it back to use the
			// existing code-path.
			if req.Question[0].Qtype == dns.TypeSRV && strings.HasPrefix(labels[i], "_") {
				parts = labels[:i+1]
				suffixes = labels[i+1:]
				done = true
			}
		}
	}

	return queryType, parts, suffixes
}

// trimDomainFromQuestionName returns the question name without the domain suffix.
func trimDomainFromQuestionName(questionName, domain, altDomain string) string {
	qName := strings.ToLower(dns.Fqdn(questionName))
	longer := domain
	shorter := altDomain

	if len(shorter) > len(longer) {
		longer, shorter = shorter, longer
	}

	if strings.HasSuffix(qName, "."+strings.TrimLeft(longer, ".")) {
		return strings.TrimSuffix(qName, longer)
	}
	return strings.TrimSuffix(qName, shorter)
}

// getQueryTypeFromLabels returns the query type from the labels.
func getQueryTypeFromLabels(label string) discovery.QueryType {
	switch label {
	case "service":
		return discovery.QueryTypeService
	case "connect":
		return discovery.QueryTypeConnect
	case "virtual":
		return discovery.QueryTypeVirtual
	case "ingress":
		return discovery.QueryTypeIngress
	case "node":
		return discovery.QueryTypeNode
	case "query":
		return discovery.QueryTypePreparedQuery
	case "workload":
		return discovery.QueryTypeWorkload
	default:
		return discovery.QueryTypeInvalid
	}
}
