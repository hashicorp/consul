// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"

	"github.com/hashicorp/consul/agent/discovery"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/dnsutil"
)

// messageSerializer is the high level orchestrator for generating the Answer,
// Extra, and Ns records for a DNS response.
type messageSerializer struct{}

// serializeOptions are the options for serializing a discovery.Result into a DNS message.
type serializeOptions struct {
	req                         *dns.Msg
	reqCtx                      Context
	query                       *discovery.Query
	results                     []*discovery.Result
	resp                        *dns.Msg
	cfg                         *RouterDynamicConfig
	responseDomain              string
	remoteAddress               net.Addr
	maxRecursionLevel           int
	dnsRecordMaker              dnsRecordMaker
	translateAddressFunc        func(dc string, addr string, taggedAddresses map[string]string, accept dnsutil.TranslateAddressAccept) string
	translateServiceAddressFunc func(dc string, address string, taggedAddresses map[string]structs.ServiceAddress, accept dnsutil.TranslateAddressAccept) string
	resolveCnameFunc            func(cfgContext *RouterDynamicConfig, name string, reqCtx Context, remoteAddress net.Addr, maxRecursionLevel int) []dns.RR
}

// serializeQueryResults converts a discovery.Result into a DNS message.
func (d messageSerializer) serialize(opts *serializeOptions) (*dns.Msg, error) {
	resp := new(dns.Msg)
	resp.SetReply(opts.req)
	resp.Compress = !opts.cfg.DisableCompression
	resp.Authoritative = true
	resp.RecursionAvailable = canRecurse(opts.cfg)
	opts.resp = resp

	qType := opts.req.Question[0].Qtype
	reqType := parseRequestType(opts.req)

	// Always add the SOA record if requested.
	if qType == dns.TypeSOA {
		resp.Answer = append(resp.Answer, opts.dnsRecordMaker.makeSOA(opts.responseDomain, opts.cfg))
	}

	switch {
	case qType == dns.TypeSOA, reqType == requestTypeAddress:
		for _, result := range opts.results {
			for _, port := range getPortsFromResult(result) {
				ans, ex, ns := d.getAnswerExtraAndNs(serializeToGetAnswerExtraAndNsOptions(opts, result, port))
				resp.Answer = append(resp.Answer, ans...)
				resp.Extra = append(resp.Extra, ex...)
				resp.Ns = append(resp.Ns, ns...)
			}
		}
	case qType == dns.TypeSRV:
		handled := make(map[string]struct{})
		for _, result := range opts.results {
			for _, port := range getPortsFromResult(result) {

				// Avoid duplicate entries, possible if a node has
				// the same service the same port, etc.

				// The datacenter should be empty during translation if it is a peering lookup.
				// This should be fine because we should always prefer the WAN address.

				address := ""
				if result.Service != nil {
					address = result.Service.Address
				} else {
					address = result.Node.Address
				}
				tuple := fmt.Sprintf("%s:%s:%d", result.Node.Name, address, port.Number)
				if _, ok := handled[tuple]; ok {
					continue
				}
				handled[tuple] = struct{}{}

				ans, ex, ns := d.getAnswerExtraAndNs(serializeToGetAnswerExtraAndNsOptions(opts, result, port))
				resp.Answer = append(resp.Answer, ans...)
				resp.Extra = append(resp.Extra, ex...)
				resp.Ns = append(resp.Ns, ns...)
			}
		}
	default:
		// default will send it to where it does some de-duping while it calls getAnswerExtraAndNs and recurses.
		d.appendResultsToDNSResponse(opts)
	}

	if opts.query != nil && opts.query.QueryType != discovery.QueryTypeVirtual &&
		len(resp.Answer) == 0 && len(resp.Extra) == 0 {
		return nil, discovery.ErrNoData
	}

	return resp, nil
}

// appendResultsToDNSResponse builds dns message from the discovery results and
// appends them to the dns response.
func (d messageSerializer) appendResultsToDNSResponse(opts *serializeOptions) {

	// Always add the SOA record if requested.
	if opts.req.Question[0].Qtype == dns.TypeSOA {
		opts.resp.Answer = append(opts.resp.Answer, opts.dnsRecordMaker.makeSOA(opts.responseDomain, opts.cfg))
	}

	handled := make(map[string]struct{})
	var answerCNAME []dns.RR = nil

	count := 0
	for _, result := range opts.results {
		for _, port := range getPortsFromResult(result) {

			// Add the node record
			had_answer := false
			ans, extra, _ := d.getAnswerExtraAndNs(serializeToGetAnswerExtraAndNsOptions(opts, result, port))
			opts.resp.Extra = append(opts.resp.Extra, extra...)

			if len(ans) == 0 {
				continue
			}

			// Avoid duplicate entries, possible if a node has
			// the same service on multiple ports, etc.
			if _, ok := handled[ans[0].String()]; ok {
				continue
			}
			handled[ans[0].String()] = struct{}{}

			switch ans[0].(type) {
			case *dns.CNAME:
				// keep track of the first CNAME + associated RRs but don't add to the resp.Answer yet
				// this will only be added if no non-CNAME RRs are found
				if len(answerCNAME) == 0 {
					answerCNAME = ans
				}
			default:
				opts.resp.Answer = append(opts.resp.Answer, ans...)
				had_answer = true
			}

			if had_answer {
				count++
				if count == opts.cfg.ARecordLimit {
					// We stop only if greater than 0 or we reached the limit
					return
				}
			}
		}
	}
	if len(opts.resp.Answer) == 0 && len(answerCNAME) > 0 {
		opts.resp.Answer = answerCNAME
	}
}

// getAnswerExtraAndNsOptions are the options for getting the Answer, Extra, and Ns records for a DNS response.
type getAnswerExtraAndNsOptions struct {
	port                        discovery.Port
	result                      *discovery.Result
	req                         *dns.Msg
	reqCtx                      Context
	query                       *discovery.Query
	results                     []*discovery.Result
	resp                        *dns.Msg
	cfg                         *RouterDynamicConfig
	responseDomain              string
	remoteAddress               net.Addr
	maxRecursionLevel           int
	ttl                         uint32
	dnsRecordMaker              dnsRecordMaker
	translateAddressFunc        func(dc string, addr string, taggedAddresses map[string]string, accept dnsutil.TranslateAddressAccept) string
	translateServiceAddressFunc func(dc string, address string, taggedAddresses map[string]structs.ServiceAddress, accept dnsutil.TranslateAddressAccept) string
	resolveCnameFunc            func(cfgContext *RouterDynamicConfig, name string, reqCtx Context, remoteAddress net.Addr, maxRecursionLevel int) []dns.RR
}

// getAnswerAndExtra creates the dns answer and extra from discovery results.
func (d messageSerializer) getAnswerExtraAndNs(opts *getAnswerExtraAndNsOptions) (answer []dns.RR, extra []dns.RR, ns []dns.RR) {
	serviceAddress, nodeAddress := d.getServiceAndNodeAddresses(opts)
	qName := opts.req.Question[0].Name
	ttlLookupName := qName
	if opts.query != nil {
		ttlLookupName = opts.query.QueryPayload.Name
	}

	opts.ttl = getTTLForResult(ttlLookupName, opts.result.DNS.TTL, opts.query, opts.cfg)

	qType := opts.req.Question[0].Qtype

	// TODO (v2-dns): skip records that refer to a workload/node that don't have a valid DNS name.

	// Special case responses
	switch {
	// PTR requests are first since they are a special case of domain overriding question type
	case parseRequestType(opts.req) == requestTypeIP:
		ptrTarget := ""
		if opts.result.Type == discovery.ResultTypeNode {
			ptrTarget = opts.result.Node.Name
		} else if opts.result.Type == discovery.ResultTypeService {
			ptrTarget = opts.result.Service.Name
		}

		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: qName, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0},
			Ptr: canonicalNameForResult(opts.result.Type, ptrTarget, opts.responseDomain, opts.result.Tenancy, opts.port.Name),
		}
		answer = append(answer, ptr)
	case qType == dns.TypeNS:
		resultType := opts.result.Type
		target := opts.result.Node.Name
		if parseRequestType(opts.req) == requestTypeConsul && resultType == discovery.ResultTypeService {
			resultType = discovery.ResultTypeNode
		}
		fqdn := canonicalNameForResult(resultType, target, opts.responseDomain, opts.result.Tenancy, opts.port.Name)
		extraRecord := opts.dnsRecordMaker.makeIPBasedRecord(fqdn, nodeAddress, opts.ttl)

		answer = append(answer, opts.dnsRecordMaker.makeNS(opts.responseDomain, fqdn, opts.ttl))
		extra = append(extra, extraRecord)
	case qType == dns.TypeSOA:
		// to be returned in the result.
		fqdn := canonicalNameForResult(opts.result.Type, opts.result.Node.Name, opts.responseDomain, opts.result.Tenancy, opts.port.Name)
		extraRecord := opts.dnsRecordMaker.makeIPBasedRecord(fqdn, nodeAddress, opts.ttl)

		ns = append(ns, opts.dnsRecordMaker.makeNS(opts.responseDomain, fqdn, opts.ttl))
		extra = append(extra, extraRecord)
	case qType == dns.TypeSRV:
		// We put A/AAAA/CNAME records in the additional section for SRV requests
		a, e := d.getAnswerExtrasForAddressAndTarget(nodeAddress, serviceAddress, opts)
		answer = append(answer, a...)
		extra = append(extra, e...)

	default:
		a, e := d.getAnswerExtrasForAddressAndTarget(nodeAddress, serviceAddress, opts)
		answer = append(answer, a...)
		extra = append(extra, e...)
	}

	a, e := getAnswerAndExtraTXT(opts.req, opts.cfg, qName, opts.result, opts.ttl,
		opts.responseDomain, opts.query, &opts.port, opts.dnsRecordMaker)
	answer = append(answer, a...)
	extra = append(extra, e...)
	return
}

// getServiceAndNodeAddresses returns the service and node addresses from a discovery result.
func (d messageSerializer) getServiceAndNodeAddresses(opts *getAnswerExtraAndNsOptions) (*dnsAddress, *dnsAddress) {
	addrTranslate := dnsutil.TranslateAddressAcceptDomain
	if opts.req.Question[0].Qtype == dns.TypeA {
		addrTranslate |= dnsutil.TranslateAddressAcceptIPv4
	} else if opts.req.Question[0].Qtype == dns.TypeAAAA {
		addrTranslate |= dnsutil.TranslateAddressAcceptIPv6
	} else {
		addrTranslate |= dnsutil.TranslateAddressAcceptAny
	}

	// The datacenter should be empty during translation if it is a peering lookup.
	// This should be fine because we should always prefer the WAN address.
	serviceAddress := newDNSAddress("")
	if opts.result.Service != nil {
		sa := opts.translateServiceAddressFunc(opts.result.Tenancy.Datacenter,
			opts.result.Service.Address, getServiceAddressMapFromLocationMap(opts.result.Service.TaggedAddresses),
			addrTranslate)
		serviceAddress = newDNSAddress(sa)
	}
	nodeAddress := newDNSAddress("")
	if opts.result.Node != nil {
		na := opts.translateAddressFunc(opts.result.Tenancy.Datacenter, opts.result.Node.Address,
			getStringAddressMapFromTaggedAddressMap(opts.result.Node.TaggedAddresses), addrTranslate)
		nodeAddress = newDNSAddress(na)
	}
	return serviceAddress, nodeAddress
}

// getAnswerExtrasForAddressAndTarget creates the dns answer and extra from nodeAddress and serviceAddress dnsAddress pairs.
func (d messageSerializer) getAnswerExtrasForAddressAndTarget(nodeAddress *dnsAddress,
	serviceAddress *dnsAddress, opts *getAnswerExtraAndNsOptions) (answer []dns.RR, extra []dns.RR) {
	qName := opts.req.Question[0].Name
	reqType := parseRequestType(opts.req)

	switch {
	case (reqType == requestTypeAddress || opts.result.Type == discovery.ResultTypeVirtual) &&
		serviceAddress.IsEmptyString() && nodeAddress.IsIP():
		a, e := getAnswerExtrasForIP(qName, nodeAddress, opts.req.Question[0],
			reqType, opts.result, opts.ttl, opts.responseDomain, &opts.port, opts.dnsRecordMaker)
		answer = append(answer, a...)
		extra = append(extra, e...)

	case opts.result.Type == discovery.ResultTypeNode && nodeAddress.IsIP():
		canonicalNodeName := canonicalNameForResult(opts.result.Type,
			opts.result.Node.Name, opts.responseDomain, opts.result.Tenancy, opts.port.Name)
		a, e := getAnswerExtrasForIP(canonicalNodeName, nodeAddress, opts.req.Question[0], reqType,
			opts.result, opts.ttl, opts.responseDomain, &opts.port, opts.dnsRecordMaker)
		answer = append(answer, a...)
		extra = append(extra, e...)

	case opts.result.Type == discovery.ResultTypeNode && !nodeAddress.IsIP():
		a, e := d.makeRecordFromFQDN(serviceAddress.FQDN(), opts)
		answer = append(answer, a...)
		extra = append(extra, e...)

	case serviceAddress.IsEmptyString() && nodeAddress.IsEmptyString():
		return nil, nil

	// There is no service address and the node address is an IP
	case serviceAddress.IsEmptyString() && nodeAddress.IsIP():
		resultType := discovery.ResultTypeNode
		if opts.result.Type == discovery.ResultTypeWorkload {
			resultType = discovery.ResultTypeWorkload
		}
		canonicalNodeName := canonicalNameForResult(resultType, opts.result.Node.Name,
			opts.responseDomain, opts.result.Tenancy, opts.port.Name)
		a, e := getAnswerExtrasForIP(canonicalNodeName, nodeAddress, opts.req.Question[0],
			reqType, opts.result, opts.ttl, opts.responseDomain, &opts.port, opts.dnsRecordMaker)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// There is no service address and the node address is a FQDN (external service)
	case serviceAddress.IsEmptyString():
		a, e := d.makeRecordFromFQDN(nodeAddress.FQDN(), opts)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The service address is an IP
	case serviceAddress.IsIP():
		canonicalServiceName := canonicalNameForResult(discovery.ResultTypeService,
			opts.result.Service.Name, opts.responseDomain, opts.result.Tenancy, opts.port.Name)
		a, e := getAnswerExtrasForIP(canonicalServiceName, serviceAddress,
			opts.req.Question[0], reqType, opts.result, opts.ttl, opts.responseDomain, &opts.port, opts.dnsRecordMaker)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// If the service address is a CNAME for the service we are looking
	// for then use the node address.
	case serviceAddress.FQDN() == opts.req.Question[0].Name && nodeAddress.IsIP():
		canonicalNodeName := canonicalNameForResult(discovery.ResultTypeNode,
			opts.result.Node.Name, opts.responseDomain, opts.result.Tenancy, opts.port.Name)
		a, e := getAnswerExtrasForIP(canonicalNodeName, nodeAddress, opts.req.Question[0],
			reqType, opts.result, opts.ttl, opts.responseDomain, &opts.port, opts.dnsRecordMaker)
		answer = append(answer, a...)
		extra = append(extra, e...)

	// The service address is a FQDN (internal or external service name)
	default:
		a, e := d.makeRecordFromFQDN(serviceAddress.FQDN(), opts)
		answer = append(answer, a...)
		extra = append(extra, e...)
	}

	return
}

// makeRecordFromFQDN creates a DNS record from a FQDN.
func (d messageSerializer) makeRecordFromFQDN(fqdn string, opts *getAnswerExtraAndNsOptions) ([]dns.RR, []dns.RR) {
	edns := opts.req.IsEdns0() != nil
	q := opts.req.Question[0]

	more := opts.resolveCnameFunc(opts.cfg, dns.Fqdn(fqdn), opts.reqCtx, opts.remoteAddress, opts.maxRecursionLevel)
	var additional []dns.RR
	extra := 0
MORE_REC:
	for _, rr := range more {
		switch rr.Header().Rrtype {
		case dns.TypeCNAME, dns.TypeA, dns.TypeAAAA, dns.TypeTXT:
			// set the TTL manually
			rr.Header().Ttl = opts.ttl
			additional = append(additional, rr)

			extra++
			if extra == maxRecurseRecords && !edns {
				break MORE_REC
			}
		}
	}

	if q.Qtype == dns.TypeSRV {
		answer := opts.dnsRecordMaker.makeSRV(q.Name, fqdn, uint16(opts.result.DNS.Weight), opts.ttl, &opts.port)
		return []dns.RR{answer}, additional
	}

	address := ""
	if opts.result.Service != nil && opts.result.Service.Address != "" {
		address = opts.result.Service.Address
	} else if opts.result.Node != nil {
		address = opts.result.Node.Address
	}

	answers := []dns.RR{
		opts.dnsRecordMaker.makeCNAME(q.Name, address, opts.ttl),
	}
	answers = append(answers, additional...)

	return answers, nil
}

// getAnswerAndExtraTXT determines whether a TXT needs to be create and then
// returns the TXT record in the answer or extra depending on the question type.
func getAnswerAndExtraTXT(req *dns.Msg, cfg *RouterDynamicConfig, qName string,
	result *discovery.Result, ttl uint32, domain string, query *discovery.Query,
	port *discovery.Port, maker dnsRecordMaker) (answer []dns.RR, extra []dns.RR) {
	if !shouldAppendTXTRecord(query, cfg, req) {
		return
	}
	recordHeaderName := qName
	serviceAddress := newDNSAddress("")
	if result.Service != nil {
		serviceAddress = newDNSAddress(result.Service.Address)
	}
	if result.Type != discovery.ResultTypeNode &&
		result.Type != discovery.ResultTypeVirtual &&
		!serviceAddress.IsInternalFQDN(domain) &&
		!serviceAddress.IsExternalFQDN(domain) {
		recordHeaderName = canonicalNameForResult(discovery.ResultTypeNode, result.Node.Name,
			domain, result.Tenancy, port.Name)
	}
	qType := req.Question[0].Qtype
	generateMeta := false
	metaInAnswer := false
	if qType == dns.TypeANY || qType == dns.TypeTXT {
		generateMeta = true
		metaInAnswer = true
	} else if cfg.NodeMetaTXT {
		generateMeta = true
	}

	// Do not generate txt records if we don't have to: https://github.com/hashicorp/consul/pull/5272
	if generateMeta {
		meta := maker.makeTXT(recordHeaderName, result.Metadata, ttl)
		if metaInAnswer {
			answer = append(answer, meta...)
		} else {
			extra = append(extra, meta...)
		}
	}
	return answer, extra
}

// shouldAppendTXTRecord determines whether a TXT record should be appended to the response.
func shouldAppendTXTRecord(query *discovery.Query, cfg *RouterDynamicConfig, req *dns.Msg) bool {
	qType := req.Question[0].Qtype
	switch {
	// Node records
	case query != nil && query.QueryType == discovery.QueryTypeNode && (cfg.NodeMetaTXT || qType == dns.TypeANY || qType == dns.TypeTXT):
		return true
	// Service records
	case query != nil && query.QueryType == discovery.QueryTypeService && cfg.NodeMetaTXT && qType == dns.TypeSRV:
		return true
	// Prepared query records
	case query != nil && query.QueryType == discovery.QueryTypePreparedQuery && cfg.NodeMetaTXT && qType == dns.TypeSRV:
		return true
	}
	return false
}

// getAnswerExtrasForIP creates the dns answer and extra from IP dnsAddress pairs.
func getAnswerExtrasForIP(name string, addr *dnsAddress, question dns.Question,
	reqType requestType, result *discovery.Result, ttl uint32, domain string,
	port *discovery.Port, maker dnsRecordMaker) (answer []dns.RR, extra []dns.RR) {
	qType := question.Qtype
	canReturnARecord := qType == dns.TypeSRV || qType == dns.TypeA || qType == dns.TypeANY || qType == dns.TypeNS || qType == dns.TypeTXT
	canReturnAAAARecord := qType == dns.TypeSRV || qType == dns.TypeAAAA || qType == dns.TypeANY || qType == dns.TypeNS || qType == dns.TypeTXT
	if reqType != requestTypeAddress && result.Type != discovery.ResultTypeVirtual {
		switch {
		// check IPV4
		case addr.IsIP() && addr.IsIPV4() && !canReturnARecord,
			// check IPV6
			addr.IsIP() && !addr.IsIPV4() && !canReturnAAAARecord:
			return
		}
	}

	// Have to pass original question name here even if the system has recursed
	// and stripped off the domain suffix.
	recHdrName := question.Name
	if qType == dns.TypeSRV {
		nameSplit := strings.Split(name, ".")
		if len(nameSplit) > 1 && nameSplit[1] == addrLabel {
			recHdrName = name
		} else {
			recHdrName = name
		}
		name = question.Name
	}

	if reqType != requestTypeAddress && qType == dns.TypeSRV {
		if result.Type == discovery.ResultTypeService && addr.IsIP() && result.Node.Address != addr.String() {
			// encode the ip to be used in the header of the A/AAAA record
			// as well as the target of the SRV record.
			recHdrName = encodeIPAsFqdn(result, addr.IP(), domain)
		}
		if result.Type == discovery.ResultTypeWorkload {
			recHdrName = canonicalNameForResult(result.Type, result.Node.Name, domain, result.Tenancy, port.Name)
		}
		srv := maker.makeSRV(name, recHdrName, uint16(result.DNS.Weight), ttl, port)
		answer = append(answer, srv)
	}

	record := maker.makeIPBasedRecord(recHdrName, addr, ttl)

	isARecordWhenNotExplicitlyQueried := record.Header().Rrtype == dns.TypeA && qType != dns.TypeA && qType != dns.TypeANY
	isAAAARecordWhenNotExplicitlyQueried := record.Header().Rrtype == dns.TypeAAAA && qType != dns.TypeAAAA && qType != dns.TypeANY

	// For explicit A/AAAA queries, we must only return those records in the answer section.
	if isARecordWhenNotExplicitlyQueried ||
		isAAAARecordWhenNotExplicitlyQueried {
		extra = append(extra, record)
	} else {
		answer = append(answer, record)
	}

	return
}

// getPortsFromResult returns the ports from a discovery result.
func getPortsFromResult(result *discovery.Result) []discovery.Port {
	if len(result.Ports) > 0 {
		return result.Ports
	}
	// return one record.
	return []discovery.Port{{}}
}

// encodeIPAsFqdn encodes an IP address as a FQDN.
func encodeIPAsFqdn(result *discovery.Result, ip net.IP, responseDomain string) string {
	ipv4 := ip.To4()
	ipStr := hex.EncodeToString(ip)
	if ipv4 != nil {
		ipStr = ipStr[len(ipStr)-(net.IPv4len*2):]
	}
	if result.Tenancy.PeerName != "" {
		// Exclude the datacenter from the FQDN on the addr for peers.
		// This technically makes no difference, since the addr endpoint ignores the DC
		// component of the request, but do it anyway for a less confusing experience.
		return fmt.Sprintf("%s.addr.%s", ipStr, responseDomain)
	}
	return fmt.Sprintf("%s.addr.%s.%s", ipStr, result.Tenancy.Datacenter, responseDomain)
}

// canonicalNameForResult returns the canonical name for a discovery result.
func canonicalNameForResult(resultType discovery.ResultType, target, domain string,
	tenancy discovery.ResultTenancy, portName string) string {
	switch resultType {
	case discovery.ResultTypeService:
		if tenancy.Namespace != "" {
			return fmt.Sprintf("%s.%s.%s.%s.%s", target, "service", tenancy.Namespace, tenancy.Datacenter, domain)
		}
		return fmt.Sprintf("%s.%s.%s.%s", target, "service", tenancy.Datacenter, domain)
	case discovery.ResultTypeNode:
		if tenancy.PeerName != "" && tenancy.Partition != "" {
			// We must return a more-specific DNS name for peering so
			// that there is no ambiguity with lookups.
			// Nodes are always registered in the default namespace, so
			// the `.ns` qualifier is not required.
			return fmt.Sprintf("%s.node.%s.peer.%s.ap.%s",
				target,
				tenancy.PeerName,
				tenancy.Partition,
				domain)
		}
		if tenancy.PeerName != "" {
			// We must return a more-specific DNS name for peering so
			// that there is no ambiguity with lookups.
			return fmt.Sprintf("%s.node.%s.peer.%s",
				target,
				tenancy.PeerName,
				domain)
		}
		// Return a simpler format for non-peering nodes.
		return fmt.Sprintf("%s.node.%s.%s", target, tenancy.Datacenter, domain)
	case discovery.ResultTypeWorkload:
		// TODO (v2-dns): it doesn't appear this is being used to return a result. Need to investigate and refactor
		if portName != "" {
			return fmt.Sprintf("%s.port.%s.workload.%s.ns.%s.ap.%s", portName, target, tenancy.Namespace, tenancy.Partition, domain)
		}
		return fmt.Sprintf("%s.workload.%s.ns.%s.ap.%s", target, tenancy.Namespace, tenancy.Partition, domain)
	}
	return ""
}

// getServiceAddressMapFromLocationMap converts a map of Location to a map of ServiceAddress.
func getServiceAddressMapFromLocationMap(taggedAddresses map[string]*discovery.TaggedAddress) map[string]structs.ServiceAddress {
	taggedServiceAddresses := make(map[string]structs.ServiceAddress, len(taggedAddresses))
	for k, v := range taggedAddresses {
		taggedServiceAddresses[k] = structs.ServiceAddress{
			Address: v.Address,
			Port:    int(v.Port.Number),
		}
	}
	return taggedServiceAddresses
}

// getStringAddressMapFromTaggedAddressMap converts a map of Location to a map of string.
func getStringAddressMapFromTaggedAddressMap(taggedAddresses map[string]*discovery.TaggedAddress) map[string]string {
	taggedServiceAddresses := make(map[string]string, len(taggedAddresses))
	for k, v := range taggedAddresses {
		taggedServiceAddresses[k] = v.Address
	}
	return taggedServiceAddresses
}

// getTTLForResult returns the TTL for a given result.
func getTTLForResult(name string, overrideTTL *uint32, query *discovery.Query, cfg *RouterDynamicConfig) uint32 {
	// In the case we are not making a discovery query, such as addr. or arpa. lookups,
	// use the node TTL by convention
	if query == nil {
		return uint32(cfg.NodeTTL / time.Second)
	}

	if overrideTTL != nil {
		// If a result was provided with an override, use that. This is the case for some prepared queries.
		return *overrideTTL
	}

	switch query.QueryType {
	case discovery.QueryTypeService, discovery.QueryTypePreparedQuery:
		ttl, ok := cfg.GetTTLForService(name)
		if ok {
			return uint32(ttl / time.Second)
		}
		fallthrough
	default:
		return uint32(cfg.NodeTTL / time.Second)
	}
}

// serializeToGetAnswerExtraAndNsOptions converts serializeOptions to getAnswerExtraAndNsOptions.
func serializeToGetAnswerExtraAndNsOptions(opts *serializeOptions,
	result *discovery.Result, port discovery.Port) *getAnswerExtraAndNsOptions {
	return &getAnswerExtraAndNsOptions{
		port:                        port,
		result:                      result,
		req:                         opts.req,
		reqCtx:                      opts.reqCtx,
		query:                       opts.query,
		results:                     opts.results,
		resp:                        opts.resp,
		cfg:                         opts.cfg,
		responseDomain:              opts.responseDomain,
		remoteAddress:               opts.remoteAddress,
		maxRecursionLevel:           opts.maxRecursionLevel,
		translateAddressFunc:        opts.translateAddressFunc,
		translateServiceAddressFunc: opts.translateServiceAddressFunc,
		resolveCnameFunc:            opts.resolveCnameFunc,
		dnsRecordMaker:              opts.dnsRecordMaker,
	}
}
