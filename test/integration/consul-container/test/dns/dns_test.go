// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"fmt"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbdns"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

func register(t *testing.T, c *api.Client, reg *api.AgentServiceRegistration) {
	t.Helper()
	require.NoError(t, c.Agent().ServiceRegister(reg))
}

// TestDNS_Lookups covers the record types and lookup forms served for services,
// nodes, Connect and ingress, using a default agent configuration.
func TestDNS_Lookups(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	c := e.client

	nodeName, err := c.Agent().NodeName()
	require.NoError(t, err)

	register(t, c, &api.AgentServiceRegistration{ID: "web-1", Name: "web", Address: "10.0.0.1", Port: 8080, Tags: []string{"primary"}})
	register(t, c, &api.AgentServiceRegistration{ID: "web-2", Name: "web", Address: "10.0.0.2", Port: 8081, Tags: []string{"secondary"}})
	register(t, c, &api.AgentServiceRegistration{ID: "web6-1", Name: "web6", Address: "2001:db8::1", Port: 8082})
	register(t, c, &api.AgentServiceRegistration{
		ID: "native-1", Name: "native", Address: "10.2.0.1", Port: 8083,
		Connect: &api.AgentServiceConnect{Native: true},
	})

	// A node without an agent, to control its address and metadata.
	_, err = c.Catalog().Register(&api.CatalogRegistration{
		Node:     "ext-node",
		Address:  "192.0.2.50",
		NodeMeta: map[string]string{"rack": "r1"},
	}, nil)
	require.NoError(t, err)

	t.Run("A record for service", func(t *testing.T) {
		e.query(t, "web.service.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, dns.RcodeSuccess, resp.Rcode)
			require.ElementsMatch(r, []string{"10.0.0.1", "10.0.0.2"}, dnsIPs(resp))
		})
	})

	t.Run("explicit datacenter form", func(t *testing.T) {
		e.query(t, "web.service.dc1.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.ElementsMatch(r, []string{"10.0.0.1", "10.0.0.2"}, dnsIPs(resp))
		})
	})

	t.Run("tag filter", func(t *testing.T) {
		e.query(t, "primary.web.service.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{"10.0.0.1"}, dnsIPs(resp))
		})
	})

	t.Run("SRV record carries port and target addresses", func(t *testing.T) {
		e.query(t, "web.service.consul", dns.TypeSRV, 0, func(r *retry.R, resp *dns.Msg) {
			require.ElementsMatch(r, []uint16{8080, 8081}, srvPorts(resp))

			// Targets are resolved in the additional section (alongside node TXT records).
			var extraIPs []string
			for _, rr := range resp.Extra {
				if a, ok := rr.(*dns.A); ok {
					extraIPs = append(extraIPs, a.A.String())
				}
			}
			require.ElementsMatch(r, []string{"10.0.0.1", "10.0.0.2"}, extraIPs)
		})
	})

	t.Run("RFC 2782 SRV form with tag", func(t *testing.T) {
		e.query(t, "_web._primary.service.consul", dns.TypeSRV, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []uint16{8080}, srvPorts(resp))
		})
	})

	t.Run("AAAA record", func(t *testing.T) {
		e.query(t, "web6.service.consul", dns.TypeAAAA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{"2001:db8::1"}, dnsIPs(resp))
		})
	})

	t.Run("node A record", func(t *testing.T) {
		node, _, err := c.Catalog().Node(nodeName, nil)
		require.NoError(t, err)
		e.query(t, nodeName+".node.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{node.Node.Address}, dnsIPs(resp))
		})
	})

	t.Run("node TXT record exposes node metadata", func(t *testing.T) {
		e.query(t, "ext-node.node.consul", dns.TypeTXT, 0, func(r *retry.R, resp *dns.Msg) {
			var txt []string
			for _, rr := range resp.Answer {
				if v, ok := rr.(*dns.TXT); ok {
					txt = append(txt, v.Txt...)
				}
			}
			require.Contains(r, txt, "rack=r1")
		})
	})

	t.Run("PTR reverse lookup of node address", func(t *testing.T) {
		e.query(t, "50.2.0.192.in-addr.arpa.", dns.TypePTR, 0, func(r *retry.R, resp *dns.Msg) {
			require.Len(r, resp.Answer, 1)
			ptr, ok := resp.Answer[0].(*dns.PTR)
			require.True(r, ok)
			require.Equal(r, "ext-node.node.dc1.consul.", ptr.Ptr)
		})
	})

	t.Run("unknown service is NXDOMAIN", func(t *testing.T) {
		e.query(t, "does-not-exist.service.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, dns.RcodeNameError, resp.Rcode)
			require.Empty(r, resp.Answer)
		})
	})

	t.Run("connect lookup resolves Connect-capable instances", func(t *testing.T) {
		e.query(t, "native.connect.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{"10.2.0.1"}, dnsIPs(resp))
		})
	})

	t.Run("ingress lookup resolves to the gateway", func(t *testing.T) {
		ok, _, err := c.ConfigEntries().Set(&api.IngressGatewayConfigEntry{
			Kind: api.IngressGateway,
			Name: "ingress",
			Listeners: []api.IngressListener{{
				Port:     9090,
				Protocol: "tcp",
				Services: []api.IngressService{{Name: "web"}},
			}},
		}, nil)
		require.NoError(t, err)
		require.True(t, ok)

		register(t, c, &api.AgentServiceRegistration{
			Kind: api.ServiceKindIngressGateway, ID: "ingress-1", Name: "ingress", Address: "10.3.0.1", Port: 9090,
		})

		e.query(t, "web.ingress.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{"10.3.0.1"}, dnsIPs(resp))
		})
	})
}

// TestDNS_ServiceHealthFiltering verifies that DNS reflects health: critical
// instances are never returned, and warning instances are returned by default.
func TestDNS_ServiceHealthFiltering(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	c := e.client

	for i, status := range []string{api.HealthPassing, api.HealthWarning, api.HealthCritical} {
		id := fmt.Sprintf("mixed-%s", status)
		register(t, c, &api.AgentServiceRegistration{
			ID: id, Name: "mixed", Address: fmt.Sprintf("10.1.0.%d", i+1), Port: 9000 + i,
			Check: &api.AgentServiceCheck{TTL: "10m", Status: status},
		})
		require.NoError(t, c.Agent().UpdateTTL("service:"+id, "", status))
	}

	e.query(t, "mixed.service.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
		require.ElementsMatch(r, []string{"10.1.0.1", "10.1.0.2"}, dnsIPs(resp))
	})

	// Recovering the critical instance makes it resolvable again.
	require.NoError(t, c.Agent().UpdateTTL("service:mixed-critical", "ok", api.HealthPassing))
	e.query(t, "mixed.service.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
		require.ElementsMatch(r, []string{"10.1.0.1", "10.1.0.2", "10.1.0.3"}, dnsIPs(resp))
	})
}

// TestDNS_ResponseSizeLimits verifies UDP answer trimming: classic (non-EDNS)
// responses are capped by udp_answer_limit, EDNS0 responses are not.
func TestDNS_ResponseSizeLimits(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	c := e.client

	const instances = 6
	for i := 0; i < instances; i++ {
		register(t, c, &api.AgentServiceRegistration{
			ID: fmt.Sprintf("many-%d", i), Name: "many", Address: fmt.Sprintf("10.4.0.%d", i+1), Port: 7000 + i,
		})
	}

	e.query(t, "many.service.consul", dns.TypeA, 4096, func(r *retry.R, resp *dns.Msg) {
		require.Len(r, resp.Answer, instances)
	})

	resp, err := dnsExchange(e, "many.service.consul", dns.TypeA, 0)
	require.NoError(t, err)
	require.Len(t, resp.Answer, 3, "default udp_answer_limit is 3 for non-EDNS clients")
}

// TestDNS_ConfigOptions verifies dns_config options end to end: only_passing,
// service/node TTLs and alt_domain.
func TestDNS_ConfigOptions(t *testing.T) {
	t.Parallel()

	e := newEnv(t, `
alt_domain = "alt.test."
dns_config {
  only_passing = true
  node_ttl = "9s"
  service_ttl {
    "*" = "7s"
  }
}
`)
	c := e.client
	nodeName, err := c.Agent().NodeName()
	require.NoError(t, err)

	for i, status := range []string{api.HealthPassing, api.HealthWarning} {
		id := fmt.Sprintf("strict-%s", status)
		register(t, c, &api.AgentServiceRegistration{
			ID: id, Name: "strict", Address: fmt.Sprintf("10.5.0.%d", i+1), Port: 9100 + i,
			Check: &api.AgentServiceCheck{TTL: "10m", Status: status},
		})
		require.NoError(t, c.Agent().UpdateTTL("service:"+id, "", status))
	}

	t.Run("only_passing hides warning instances and applies service TTL", func(t *testing.T) {
		e.query(t, "strict.service.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{"10.5.0.1"}, dnsIPs(resp))
			require.Equal(r, uint32(7), resp.Answer[0].Header().Ttl)
		})
	})

	t.Run("node TTL", func(t *testing.T) {
		e.query(t, nodeName+".node.consul", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.NotEmpty(r, resp.Answer)
			require.Equal(r, uint32(9), resp.Answer[0].Header().Ttl)
		})
	})

	t.Run("alt_domain", func(t *testing.T) {
		e.query(t, "strict.service.alt.test.", dns.TypeA, 0, func(r *retry.R, resp *dns.Msg) {
			require.Equal(r, []string{"10.5.0.1"}, dnsIPs(resp))
		})
	})
}

// TestDNS_OverGRPC verifies the DNS-over-gRPC service returns the same answers
// as the UDP listener.
func TestDNS_OverGRPC(t *testing.T) {
	t.Parallel()

	e := newEnv(t, "")
	register(t, e.client, &api.AgentServiceRegistration{ID: "grpc-web-1", Name: "grpcweb", Address: "10.6.0.1", Port: 8080})

	conn := e.grpcConn()
	require.NotNil(t, conn)
	dnsClient := pbdns.NewDNSServiceClient(conn)

	req := new(dns.Msg)
	req.SetQuestion("grpcweb.service.consul.", dns.TypeA)
	packed, err := req.Pack()
	require.NoError(t, err)

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		out, err := dnsClient.Query(context.Background(), &pbdns.QueryRequest{
			Msg:      packed,
			Protocol: pbdns.Protocol_PROTOCOL_UDP,
		})
		require.NoError(r, err)

		resp := new(dns.Msg)
		require.NoError(r, resp.Unpack(out.Msg))
		require.Equal(r, []string{"10.6.0.1"}, dnsIPs(resp))
	})
}
