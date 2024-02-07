package dns

import (
	"fmt"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"
	"math"
	"strings"
)

const (
	// UDP can fit ~25 A records in a 512B response, and ~14 AAAA
	// records. Limit further to prevent unintentional configuration
	// abuse that would have a negative effect on application response
	// times.
	maxUDPAnswerLimit = 8

	defaultMaxUDPSize = 512

	// If a consumer sets a buffer size greater than this amount we will default it down
	// to this amount to ensure that consul does respond. Previously if consumer had a larger buffer
	// size than 65535 - 60 bytes (maximim 60 bytes for IP header. UDP header will be offset in the
	// trimUDP call) consul would fail to respond and the consumer timesout
	// the request.
	maxUDPDatagramSize = math.MaxUint16 - 68
)

// trimDNSResponse will trim the response for UDP and TCP
func trimDNSResponse(cfg *RouterDynamicConfig, network string, req, resp *dns.Msg, logger hclog.Logger) {
	var trimmed bool
	originalSize := resp.Len()
	originalNumRecords := len(resp.Answer)
	if network != "tcp" {
		trimmed = trimUDPResponse(req, resp, cfg.UDPAnswerLimit)
	} else {
		trimmed = trimTCPResponse(req, resp)
	}
	// Flag that there are more records to return in the UDP response
	if trimmed {
		if cfg.EnableTruncate {
			resp.Truncated = true
		}
		logger.Debug("DNS response too large, truncated",
			"protocol", network,
			"question", req.Question,
			"records", fmt.Sprintf("%d/%d", len(resp.Answer), originalNumRecords),
			"size", fmt.Sprintf("%d/%d", resp.Len(), originalSize),
		)
	}
}

// trimTCPResponse limit the MaximumSize of messages to 64k as it is the limit
// of DNS responses
func trimTCPResponse(req, resp *dns.Msg) (trimmed bool) {
	hasExtra := len(resp.Extra) > 0
	// There is some overhead, 65535 does not work
	maxSize := 65523 // 64k - 12 bytes DNS raw overhead

	// We avoid some function calls and allocations by only handling the
	// extra data when necessary.
	var index map[string]dns.RR

	// It is not possible to return more than 4k records even with compression
	// Since we are performing binary search it is not a big deal, but it
	// improves a bit performance, even with binary search
	truncateAt := 4096
	if req.Question[0].Qtype == dns.TypeSRV {
		// More than 1024 SRV records do not fit in 64k
		truncateAt = 1024
	}
	if len(resp.Answer) > truncateAt {
		resp.Answer = resp.Answer[:truncateAt]
	}
	if hasExtra {
		index = make(map[string]dns.RR, len(resp.Extra))
		indexRRs(resp.Extra, index)
	}
	truncated := false

	// This enforces the given limit on 64k, the max limit for DNS messages
	for len(resp.Answer) > 1 && resp.Len() > maxSize {
		truncated = true
		// first try to remove the NS section may be it will truncate enough
		if len(resp.Ns) != 0 {
			resp.Ns = []dns.RR{}
		}
		// More than 100 bytes, find with a binary search
		if resp.Len()-maxSize > 100 {
			bestIndex := dnsBinaryTruncate(resp, maxSize, index, hasExtra)
			resp.Answer = resp.Answer[:bestIndex]
		} else {
			resp.Answer = resp.Answer[:len(resp.Answer)-1]
		}
		if hasExtra {
			syncExtra(index, resp)
		}
	}

	return truncated
}

// trimUDPResponse makes sure a UDP response is not longer than allowed by RFC
// 1035. Enforce an arbitrary limit that can be further ratcheted down by
// config, and then make sure the response doesn't exceed 512 bytes. Any extra
// records will be trimmed along with answers.
func trimUDPResponse(req, resp *dns.Msg, udpAnswerLimit int) (trimmed bool) {
	numAnswers := len(resp.Answer)
	hasExtra := len(resp.Extra) > 0
	maxSize := defaultMaxUDPSize

	// Update to the maximum edns size
	if edns := req.IsEdns0(); edns != nil {
		if size := edns.UDPSize(); size > uint16(maxSize) {
			maxSize = int(size)
		}
	}
	// Overriding maxSize as the maxSize cannot be larger than the
	// maxUDPDatagram size. Reliability guarantees disappear > than this amount.
	if maxSize > maxUDPDatagramSize {
		maxSize = maxUDPDatagramSize
	}

	// We avoid some function calls and allocations by only handling the
	// extra data when necessary.
	var index map[string]dns.RR
	if hasExtra {
		index = make(map[string]dns.RR, len(resp.Extra))
		indexRRs(resp.Extra, index)
	}

	// This cuts UDP responses to a useful but limited number of responses.
	maxAnswers := lib.MinInt(maxUDPAnswerLimit, udpAnswerLimit)
	compress := resp.Compress
	if maxSize == defaultMaxUDPSize && numAnswers > maxAnswers {
		// We disable computation of Len ONLY for non-eDNS request (512 bytes)
		resp.Compress = false
		resp.Answer = resp.Answer[:maxAnswers]
		if hasExtra {
			syncExtra(index, resp)
		}
	}
	if maxSize == defaultMaxUDPSize && numAnswers > maxAnswers {
		// We disable computation of Len ONLY for non-eDNS request (512 bytes)
		resp.Compress = false
		resp.Answer = resp.Answer[:maxAnswers]
		if hasExtra {
			syncExtra(index, resp)
		}
	}

	// This enforces the given limit on the number bytes. The default is 512 as
	// per the RFC, but EDNS0 allows for the user to specify larger sizes. Note
	// that we temporarily switch to uncompressed so that we limit to a response
	// that will not exceed 512 bytes uncompressed, which is more conservative and
	// will allow our responses to be compliant even if some downstream server
	// uncompresses them.
	// Even when size is too big for one single record, try to send it anyway
	// (useful for 512 bytes messages). 8 is removed from maxSize to ensure that we account
	// for the udp header (8 bytes).
	for len(resp.Answer) > 1 && resp.Len() > maxSize-8 {
		// first try to remove the NS section may be it will truncate enough
		if len(resp.Ns) != 0 {
			resp.Ns = []dns.RR{}
		}
		// More than 100 bytes, find with a binary search
		if resp.Len()-maxSize > 100 {
			bestIndex := dnsBinaryTruncate(resp, maxSize, index, hasExtra)
			resp.Answer = resp.Answer[:bestIndex]
		} else {
			resp.Answer = resp.Answer[:len(resp.Answer)-1]
		}
		if hasExtra {
			syncExtra(index, resp)
		}
	}
	// For 512 non-eDNS responses, while we compute size non-compressed,
	// we send result compressed
	resp.Compress = compress
	return len(resp.Answer) < numAnswers
}

// syncExtra takes a DNS response message and sets the extra data to the most
// minimal set needed to cover the answer data. A pre-made index of RRs is given
// so that can be re-used between calls. This assumes that the extra data is
// only used to provide info for SRV records. If that's not the case, then this
// will wipe out any additional data.
func syncExtra(index map[string]dns.RR, resp *dns.Msg) {
	extra := make([]dns.RR, 0, len(resp.Answer))
	resolved := make(map[string]struct{}, len(resp.Answer))
	for _, ansRR := range resp.Answer {
		srv, ok := ansRR.(*dns.SRV)
		if !ok {
			continue
		}

		// Note that we always use lower case when using the index so
		// that compares are not case-sensitive. We don't alter the actual
		// RRs we add into the extra section, however.
		target := strings.ToLower(srv.Target)

	RESOLVE:
		if _, ok := resolved[target]; ok {
			continue
		}
		resolved[target] = struct{}{}

		extraRR, ok := index[target]
		if ok {
			extra = append(extra, extraRR)
			if cname, ok := extraRR.(*dns.CNAME); ok {
				target = strings.ToLower(cname.Target)
				goto RESOLVE
			}
		}
	}
	resp.Extra = extra
}

// dnsBinaryTruncate find the optimal number of records using a fast binary search and return
// it in order to return a DNS answer lower than maxSize parameter.
func dnsBinaryTruncate(resp *dns.Msg, maxSize int, index map[string]dns.RR, hasExtra bool) int {
	originalAnswser := resp.Answer
	startIndex := 0
	endIndex := len(resp.Answer) + 1
	for endIndex-startIndex > 1 {
		median := startIndex + (endIndex-startIndex)/2

		resp.Answer = originalAnswser[:median]
		if hasExtra {
			syncExtra(index, resp)
		}
		aLen := resp.Len()
		if aLen <= maxSize {
			if maxSize-aLen < 10 {
				// We are good, increasing will go out of bounds
				return median
			}
			startIndex = median
		} else {
			endIndex = median
		}
	}
	return startIndex
}

// indexRRs populates a map which indexes a given list of RRs by name. NOTE that
// the names are all squashed to lower case so we can perform case-insensitive
// lookups; the RRs are not modified.
func indexRRs(rrs []dns.RR, index map[string]dns.RR) {
	for _, rr := range rrs {
		name := strings.ToLower(rr.Header().Name)
		if _, ok := index[name]; !ok {
			index[name] = rr
		}
	}
}
