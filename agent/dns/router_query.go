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
func buildQueryFromDNSMessage(req *dns.Msg, domain, altDomain string,
	cfg *RouterDynamicConfig, defaultEntMeta acl.EnterpriseMeta, defaultDatacenter string) (*discovery.Query, error) {
	queryType, queryParts, querySuffixes := getQueryTypePartsAndSuffixesFromDNSMessage(req, domain, altDomain)

	queryTenancy, err := getQueryTenancy(queryType, querySuffixes, defaultEntMeta, cfg, defaultDatacenter)
	if err != nil {
		return nil, err
	}

	name, tag := getQueryNameAndTagFromParts(queryType, queryParts)

	return &discovery.Query{
		QueryType: queryType,
		QueryPayload: discovery.QueryPayload{
			Name:    name,
			Tenancy: queryTenancy,
			Tag:     tag,
			// TODO (v2-dns): what should these be?
			//PortName:   "",
			//RemoteAddr: nil,
			//DisableFailover: false,
		},
	}, nil
}

// getQueryNameAndTagFromParts returns the query name and tag from the query parts that are taken from the original dns question.
func getQueryNameAndTagFromParts(queryType discovery.QueryType, queryParts []string) (string, string) {
	switch queryType {
	case discovery.QueryTypeService:
		n := len(queryParts)
		// Support RFC 2782 style syntax
		if n == 2 && strings.HasPrefix(queryParts[1], "_") && strings.HasPrefix(queryParts[0], "_") {
			// Grab the tag since we make nuke it if it's tcp
			tag := queryParts[1][1:]

			// Treat _name._tcp.service.consul as a default, no need to filter on that tag
			if tag == "tcp" {
				tag = ""
			}

			name := queryParts[0][1:]
			// _name._tag.service.consul
			return name, tag
		}
		return queryParts[len(queryParts)-1], ""
	}
	return queryParts[len(queryParts)-1], ""
}

// getQueryTenancy returns a discovery.QueryTenancy from a DNS message.
func getQueryTenancy(queryType discovery.QueryType, querySuffixes []string,
	defaultEntMeta acl.EnterpriseMeta, cfg *RouterDynamicConfig, defaultDatacenter string) (discovery.QueryTenancy, error) {
	if queryType == discovery.QueryTypeService {
		return getQueryTenancyForService(querySuffixes, defaultEntMeta, cfg, defaultDatacenter)
	}

	locality, ok := discovery.ParseLocality(querySuffixes, defaultEntMeta, cfg.EnterpriseDNSConfig)
	if !ok {
		return discovery.QueryTenancy{}, errors.New("invalid locality")
	}

	if queryType == discovery.QueryTypeVirtual {
		if locality.Peer == "" {
			// If the peer name was not explicitly defined, fall back to the ambiguously-parsed version.
			locality.Peer = locality.PeerOrDatacenter
		}
	}

	return discovery.GetQueryTenancyBasedOnLocality(locality, defaultDatacenter)
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
		case discovery.QueryTypeService,
			discovery.QueryTypeConnect, discovery.QueryTypeVirtual, discovery.QueryTypeIngress,
			discovery.QueryTypeNode, discovery.QueryTypePreparedQuery:
			parts = labels[:i]
			suffixes = labels[i+1:]
			done = true
		case discovery.QueryTypeInvalid:
			fallthrough
		default:
			// If this is a SRV query the "service" label is optional, we add it back to use the
			// existing code-path.
			if req.Question[0].Qtype == dns.TypeSRV && strings.HasPrefix(labels[i], "_") {
				queryType = discovery.QueryTypeService
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
