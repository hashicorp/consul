// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package assert

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// DNSAddr returns the host:port, reachable from the test process, that is
// mapped to the agent's DNS (UDP) port.
func DNSAddr(t *testing.T, agent libcluster.Agent) string {
	t.Helper()

	ctx := context.Background()
	host, err := agent.GetPod().Host(ctx)
	require.NoError(t, err)
	port, err := agent.GetPod().MappedPort(ctx, "8600/udp")
	require.NoError(t, err)
	return net.JoinHostPort(host, port.Port())
}

// DNSExchange sends a single UDP query to addr. When ednsSize is non-zero an
// EDNS0 OPT record advertising that UDP payload size is attached; otherwise
// the classic 512-byte limit applies.
func DNSExchange(addr, name string, qtype uint16, ednsSize uint16) (*dns.Msg, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	if ednsSize > 0 {
		m.SetEdns0(ednsSize, false)
	}

	c := &dns.Client{Net: "udp", Timeout: 5 * time.Second}
	resp, _, err := c.Exchange(m, addr)
	return resp, err
}

// DNSAnswerIPs returns the addresses of all A/AAAA answers in resp.
func DNSAnswerIPs(resp *dns.Msg) []string {
	var out []string
	for _, rr := range resp.Answer {
		switch v := rr.(type) {
		case *dns.A:
			out = append(out, v.A.String())
		case *dns.AAAA:
			out = append(out, v.AAAA.String())
		}
	}
	return out
}
