// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"encoding/hex"
	"net"
	"strings"

	"github.com/miekg/dns"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/dnsutil"
)

// discoveryResultsFetcher is a facade for the DNS router to formulate
// and execute discovery queries.
type discoveryResultsFetcher struct{}

// getQueryOptions is a struct to hold the options for getQueryResults method.
type getQueryOptions struct {
	req           *dns.Msg
	reqCtx        Context
	qName         string
	remoteAddress net.Addr
	processor     DiscoveryQueryProcessor
	logger        hclog.Logger
	domain        string
	altDomain     string
}

// getQueryResults returns a discovery.Result from a DNS message.
func (d discoveryResultsFetcher) getQueryResults(opts *getQueryOptions) ([]*discovery.Result, *discovery.Query, error) {
	reqType := parseRequestType(opts.req)

	switch reqType {
	case requestTypeConsul:
		// This is a special case of discovery.QueryByName where we know that we need to query the consul service
		// regardless of the question name.
		query := &discovery.Query{
			QueryType: discovery.QueryTypeService,
			QueryPayload: discovery.QueryPayload{
				Name: structs.ConsulServiceName,
				Tenancy: discovery.QueryTenancy{
					// We specify the partition here so that in the case we are a client agent in a non-default partition.
					// We don't want the query processors default partition to be used.
					// This is a small hack because for V1 CE, this is not the correct default partition name, but we
					// need to add something to disambiguate the empty field.
					Partition: acl.DefaultPartitionName, //NOTE: note this won't work if we ever have V2 client agents
				},
				Limit: 3,
			},
		}

		results, err := opts.processor.QueryByName(query, discovery.Context{Token: opts.reqCtx.Token})
		return results, query, err
	case requestTypeName:
		query, err := buildQueryFromDNSMessage(opts.req, opts.reqCtx, opts.domain, opts.altDomain, opts.remoteAddress)
		if err != nil {
			opts.logger.Error("error building discovery query from DNS request", "error", err)
			return nil, query, err
		}
		results, err := opts.processor.QueryByName(query, discovery.Context{Token: opts.reqCtx.Token})

		if getErrorFromECSNotGlobalError(err) != nil {
			opts.logger.Error("error processing discovery query", "error", err)
			if structs.IsErrSamenessGroupMustBeDefaultForFailover(err) {
				return nil, query, errNameNotFound
			}
			return nil, query, err
		}
		return results, query, err
	case requestTypeIP:
		ip := dnsutil.IPFromARPA(opts.qName)
		if ip == nil {
			opts.logger.Error("error building IP from DNS request", "name", opts.qName)
			return nil, nil, errNameNotFound
		}
		results, err := opts.processor.QueryByIP(ip, discovery.Context{Token: opts.reqCtx.Token})
		return results, nil, err
	case requestTypeAddress:
		results, err := buildAddressResults(opts.req)
		if err != nil {
			opts.logger.Error("error processing discovery query", "error", err)
			return nil, nil, err
		}
		return results, nil, nil
	}

	opts.logger.Error("error parsing discovery query type", "requestType", reqType)
	return nil, nil, errInvalidQuestion
}

// buildQueryFromDNSMessage returns a discovery.Query from a DNS message.
func buildQueryFromDNSMessage(req *dns.Msg, reqCtx Context, domain, altDomain string,
	remoteAddress net.Addr) (*discovery.Query, error) {
	queryType, queryParts, querySuffixes := getQueryTypePartsAndSuffixesFromDNSMessage(req, domain, altDomain)

	queryTenancy, err := getQueryTenancy(reqCtx, queryType, querySuffixes)
	if err != nil {
		return nil, err
	}

	name, tag := getQueryNameAndTagFromParts(queryType, queryParts)

	portName := parsePort(queryParts)

	switch {
	case queryType == discovery.QueryTypeWorkload && req.Question[0].Qtype == dns.TypeSRV:
		// Currently we do not support SRV records for workloads
		return nil, errNotImplemented
	case queryType == discovery.QueryTypeInvalid, name == "":
		return nil, errInvalidQuestion
	}

	return &discovery.Query{
		QueryType: queryType,
		QueryPayload: discovery.QueryPayload{
			Name:     name,
			Tenancy:  queryTenancy,
			Tag:      tag,
			PortName: portName,
			SourceIP: getSourceIP(req, queryType, remoteAddress),
		},
	}, nil
}

// buildAddressResults returns a discovery.Result from a DNS request for addr. records.
func buildAddressResults(req *dns.Msg) ([]*discovery.Result, error) {
	domain := dns.CanonicalName(req.Question[0].Name)
	labels := dns.SplitDomainName(domain)
	hexadecimal := labels[0]

	if len(hexadecimal)/2 != 4 && len(hexadecimal)/2 != 16 {
		return nil, errNameNotFound
	}

	var ip net.IP
	ip, err := hex.DecodeString(hexadecimal)
	if err != nil {
		return nil, errNameNotFound
	}

	return []*discovery.Result{
		{
			Node: &discovery.Location{
				Address: ip.String(),
			},
			Type: discovery.ResultTypeNode, // We choose node by convention since we do not know the origin of the IP
		},
	}, nil
}

// getQueryNameAndTagFromParts returns the query name and tag from the query parts that are taken from the original dns question.
func getQueryNameAndTagFromParts(queryType discovery.QueryType, queryParts []string) (string, string) {
	n := len(queryParts)
	if n == 0 {
		return "", ""
	}

	switch queryType {
	case discovery.QueryTypeService:
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
		return queryParts[n-1], ""
	case discovery.QueryTypePreparedQuery:
		name := ""

		// If the first and last DNS query parts begin with _, this is an RFC 2782 style SRV lookup.
		// This allows for prepared query names to include "." (for backwards compatibility).
		// Otherwise, this is a standard prepared query lookup.
		if n >= 2 && strings.HasPrefix(queryParts[0], "_") && strings.HasPrefix(queryParts[n-1], "_") {
			// The last DNS query part is the protocol field (ignored).
			// All prior parts are the prepared query name or ID.
			name = strings.Join(queryParts[:n-1], ".")

			// Strip leading underscore
			name = name[1:]
		} else {
			// Allow a "." in the query name, just join all the parts.
			name = strings.Join(queryParts, ".")
		}
		return name, ""
	}
	return queryParts[n-1], ""
}

// getQueryTenancy returns a discovery.QueryTenancy from a DNS message.
func getQueryTenancy(reqCtx Context, queryType discovery.QueryType, querySuffixes []string) (discovery.QueryTenancy, error) {
	labels, ok := parseLabels(querySuffixes)
	if !ok {
		return discovery.QueryTenancy{}, errNameNotFound
	}

	// If we don't have an explicit partition/ns in the request, try the first fallback
	// which was supplied in the request context. The agent's partition will be used as the last fallback
	// later in the query processor.
	if labels.Partition == "" {
		labels.Partition = reqCtx.DefaultPartition
	}

	if labels.Namespace == "" {
		labels.Namespace = reqCtx.DefaultNamespace
	}

	// If we have a sameness group, we can return early without further data massage.
	if labels.SamenessGroup != "" {
		return discovery.QueryTenancy{
			Namespace:     labels.Namespace,
			Partition:     labels.Partition,
			SamenessGroup: labels.SamenessGroup,
			// Datacenter is not supported
		}, nil
	}

	if queryType == discovery.QueryTypeVirtual {
		if labels.Peer == "" {
			// If the peer name was not explicitly defined, fall back to the ambiguously-parsed version.
			labels.Peer = labels.PeerOrDatacenter
		}
	}

	return discovery.QueryTenancy{
		Namespace:  labels.Namespace,
		Partition:  labels.Partition,
		Peer:       labels.Peer,
		Datacenter: getEffectiveDatacenter(labels),
	}, nil
}

// getEffectiveDatacenter returns the effective datacenter from the parsed labels.
func getEffectiveDatacenter(labels *parsedLabels) string {
	switch {
	case labels.Datacenter != "":
		return labels.Datacenter
	case labels.PeerOrDatacenter != "" && labels.Peer != labels.PeerOrDatacenter:
		return labels.PeerOrDatacenter
	}
	return ""
}

// getQueryTypePartsAndSuffixesFromDNSMessage returns the query type, the parts, and suffixes of the query name.
func getQueryTypePartsAndSuffixesFromDNSMessage(req *dns.Msg, domain, altDomain string) (queryType discovery.QueryType, parts []string, suffixes []string) {
	// Get the QName without the domain suffix
	// TODO (v2-dns): we will also need to handle the "failover" and "no-failover" suffixes here.
	// They come AFTER the domain. See `stripAnyFailoverSuffix` in router.go
	qName := trimDomainFromQuestionName(req.Question[0].Name, domain, altDomain)

	// Split into the label parts
	labels := dns.SplitDomainName(qName)

	done := false
	for i := len(labels) - 1; i >= 0 && !done; i-- {
		queryType = getQueryTypeFromLabels(labels[i])
		switch queryType {
		case discovery.QueryTypeService, discovery.QueryTypeWorkload,
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
	qName := dns.CanonicalName(questionName)
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

// getSourceIP returns the source IP from the dns request.
func getSourceIP(req *dns.Msg, queryType discovery.QueryType, remoteAddr net.Addr) (sourceIP net.IP) {
	if queryType == discovery.QueryTypePreparedQuery {
		subnet := ednsSubnetForRequest(req)

		if subnet != nil {
			sourceIP = subnet.Address
		} else {
			switch v := remoteAddr.(type) {
			case *net.UDPAddr:
				sourceIP = v.IP
			case *net.TCPAddr:
				sourceIP = v.IP
			case *net.IPAddr:
				sourceIP = v.IP
			}
		}
	}
	return sourceIP
}
