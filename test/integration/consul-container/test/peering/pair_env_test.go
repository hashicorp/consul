// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peering

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libassert "github.com/hashicorp/consul/test/integration/consul-container/libs/assert"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

const (
	// acceptorPeerName is how the accepting datacenter names the dialer.
	acceptorPeerName = "to-dialer"
	// dialerPeerName is how the dialing datacenter names the acceptor.
	dialerPeerName = "to-acceptor"
)

// peeringPair is two single-server datacenters that can be peered. Peering is
// control-plane only here: no Envoy or workloads are involved, so these tests
// cover what the servers do with tokens, peerings, exports and imports.
type peeringPair struct {
	acceptor *libcluster.Cluster
	dialer   *libcluster.Cluster
	// acc and dial are clients to each datacenter's server. When ACLs are
	// enabled they carry that datacenter's bootstrap token.
	acc  *api.Client
	dial *api.Client
}

func newPeeringPair(t *testing.T, aclEnabled bool) *peeringPair {
	t.Helper()

	build := func(dc string) (*libcluster.Cluster, *api.Client) {
		cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
			NumServers: 1,
			NumClients: 0,
			BuildOpts: &libcluster.BuildOptions{
				Datacenter: dc,
				// Auto-encrypt conflicts with ACL bootstrap, so only use it without ACLs.
				InjectAutoEncryption: !aclEnabled,
				ACLEnabled:           aclEnabled,
			},
		})
		return cluster, client
	}

	acceptor, acc := build("dc1")
	dialer, dial := build("dc2")
	return &peeringPair{acceptor: acceptor, dialer: dialer, acc: acc, dial: dial}
}

// tokenClient returns a client to the given cluster's server that authenticates
// as token.
func tokenClient(t *testing.T, cluster *libcluster.Cluster, token string) *api.Client {
	t.Helper()
	c, err := cluster.Agents[0].NewClient(token, false)
	require.NoError(t, err)
	return c
}

func generateToken(c *api.Client, peerName string) (string, error) {
	res, _, err := c.Peerings().GenerateToken(context.Background(), api.PeeringGenerateTokenRequest{PeerName: peerName}, nil)
	if err != nil {
		return "", err
	}
	return res.PeeringToken, nil
}

func establish(c *api.Client, peerName, token string) error {
	_, _, err := c.Peerings().Establish(context.Background(), api.PeeringEstablishRequest{PeerName: peerName, PeeringToken: token}, nil)
	return err
}

// connect generates a token on the acceptor and establishes the peering from
// the dialer, then waits for both sides to be active. It returns the token.
func (p *peeringPair) connect(t *testing.T) string {
	t.Helper()

	token, err := generateToken(p.acc, acceptorPeerName)
	require.NoError(t, err)
	require.NoError(t, establish(p.dial, dialerPeerName, token))

	libassert.PeeringStatus(t, p.acc, acceptorPeerName, api.PeeringStateActive)
	libassert.PeeringStatus(t, p.dial, dialerPeerName, api.PeeringStateActive)
	return token
}

// peeringState reads a peering, returning nil when it does not exist.
func peeringState(t retry.TestingTB, c *api.Client, name string) *api.Peering {
	peering, _, err := c.Peerings().Read(context.Background(), name, nil)
	require.NoError(t, err)
	return peering
}

func peeringNames(t retry.TestingTB, c *api.Client) []string {
	list, _, err := c.Peerings().List(context.Background(), nil)
	require.NoError(t, err)
	var names []string
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names
}
