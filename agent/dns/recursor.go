// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"errors"
	"net"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/miekg/dns"

	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/logging"
)

type recursor struct {
	logger hclog.Logger
}

func newRecursor(logger hclog.Logger) *recursor {
	return &recursor{
		logger: logger.Named(logging.DNS),
	}
}

// handle is used to process DNS queries for externally configured servers
func (r *recursor) handle(req *dns.Msg, cfgCtx *RouterDynamicConfig, remoteAddr net.Addr) (*dns.Msg, error) {
	q := req.Question[0]

	network := "udp"
	defer func(s time.Time) {
		r.logger.Debug("request served from client",
			"question", q,
			"network", network,
			"latency", time.Since(s).String(),
			"client", remoteAddr.String(),
			"client_network", remoteAddr.Network(),
		)
	}(time.Now())

	// Switch to TCP if the client is
	if _, ok := remoteAddr.(*net.TCPAddr); ok {
		network = "tcp"
	}

	// Recursively resolve
	c := &dns.Client{Net: network, Timeout: cfgCtx.RecursorTimeout}
	var resp *dns.Msg
	var rtt time.Duration
	var err error
	for _, idx := range cfgCtx.RecursorStrategy.Indexes(len(cfgCtx.Recursors)) {
		recurseAddr := cfgCtx.Recursors[idx]
		resp, rtt, err = c.Exchange(req, recurseAddr)
		// Check if the response is valid and has the desired Response code
		if resp != nil && (resp.Rcode != dns.RcodeSuccess && resp.Rcode != dns.RcodeNameError) {
			r.logger.Debug("recurse failed for question",
				"question", q,
				"rtt", rtt,
				"recursor", recurseAddr,
				"rcode", dns.RcodeToString[resp.Rcode],
			)
			// If we still have recursors to forward the query to,
			// we move forward onto the next one else the loop ends
			continue
		} else if err == nil || (resp != nil && resp.Truncated) {
			// Compress the response; we don't know if the incoming
			// response was compressed or not, so by not compressing
			// we might generate an invalid packet on the way out.
			resp.Compress = !cfgCtx.DisableCompression

			// Forward the response
			r.logger.Debug("recurse succeeded for question",
				"question", q,
				"rtt", rtt,
				"recursor", recurseAddr,
			)
			return resp, nil
		}
		r.logger.Error("recurse failed", "error", err)
	}

	// If all resolvers fail, return a SERVFAIL message
	r.logger.Error("all resolvers failed for question from client",
		"question", q,
		"client", remoteAddr.String(),
		"client_network", remoteAddr.Network(),
	)

	return nil, errRecursionFailed
}

// formatRecursorAddress is used to add a port to the recursor if omitted.
func formatRecursorAddress(recursor string) (string, error) {
	_, _, err := net.SplitHostPort(recursor)
	var ae *net.AddrError
	if errors.As(err, &ae) {
		switch ae.Err {
		case "missing port in address":
			recursor = ipaddr.FormatAddressPort(recursor, 53)
		case "too many colons in address":
			if ip := net.ParseIP(recursor); ip != nil && ip.To4() == nil {
				recursor = ipaddr.FormatAddressPort(recursor, 53)
				break
			}
			fallthrough
		default:
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	// Get the address
	addr, err := net.ResolveTCPAddr("tcp", recursor)
	if err != nil {
		return "", err
	}

	// Return string
	return addr.String(), nil
}
