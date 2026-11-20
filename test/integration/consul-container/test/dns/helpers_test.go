// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env bundles what every DNS test needs: the cluster, an API client and the
// DNS endpoint of the client agent. Clients answer DNS by forwarding to the
// servers, so this exercises the same path an application would use.
type env struct {
	cluster *libcluster.Cluster
	client  *api.Client
	dnsAddr string
}

// newEnv creates one server + one client. clientHCL is merged into the client
// agent's configuration (use it for dns_config).
func newEnv(t *testing.T, clientHCL string) *env {
	t.Helper()

	cluster, _, client := libtopology.NewClusterWithConfig(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	}, "", clientHCL)

	return &env{
		cluster: cluster,
		client:  client,
		dnsAddr: libassert.DNSAddr(t, cluster.Agents[1]),
	}
}

// query retries a DNS query until check passes, which absorbs the delay
// between registering on the agent and the record being served.
func (e *env) query(t *testing.T, name string, qtype uint16, ednsSize uint16, check func(r *retry.R, resp *dns.Msg)) {
	t.Helper()

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		resp, err := libassert.DNSExchange(e.dnsAddr, name, qtype, ednsSize)
		require.NoError(r, err)
		check(r, resp)
	})
}

func srvPorts(resp *dns.Msg) []uint16 {
	var ports []uint16
	for _, rr := range resp.Answer {
		if srv, ok := rr.(*dns.SRV); ok {
			ports = append(ports, srv.Port)
		}
	}
	return ports
}

func dnsIPs(resp *dns.Msg) []string { return libassert.DNSAnswerIPs(resp) }

func dnsExchange(e *env, name string, qtype uint16, ednsSize uint16) (*dns.Msg, error) {
	return libassert.DNSExchange(e.dnsAddr, name, qtype, ednsSize)
}

func (e *env) grpcConn() *grpc.ClientConn { return e.cluster.Agents[1].GetGRPCConn() }
