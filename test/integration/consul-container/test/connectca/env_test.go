// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connectca

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libtopology "github.com/hashicorp/consul/test/integration/consul-container/libs/topology"
)

// env is a one server + one client datacenter.
type env struct {
	cluster *libcluster.Cluster
	server  *api.Client
	client  *api.Client
}

func newEnv(t *testing.T) *env {
	t.Helper()

	cluster, _, client := libtopology.NewCluster(t, &libtopology.ClusterConfig{
		NumServers: 1,
		NumClients: 1,
		BuildOpts: &libcluster.BuildOptions{
			Datacenter:           "dc1",
			InjectAutoEncryption: true,
		},
	})
	e := &env{cluster: cluster, server: cluster.Agents[0].GetClient(), client: client}

	// Services that need leaf certificates must be registered on the agent that issues them.
	require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: "web", Port: 8080}))
	return e
}

// shell runs a script in the client agent's container.
func (e *env) shell(t *testing.T, script string) libcluster.ExecResult {
	t.Helper()
	res, err := libcluster.ExecCombined(context.Background(), e.cluster.Agents[1], "sh", "-c", script)
	require.NoError(t, err)
	return res
}

func eventually(t *testing.T, check func(r *retry.R)) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, check)
}

// roots returns the trust bundle as the servers publish it.
func roots(t retry.TestingTB, c *api.Client) *api.CARootList {
	list, _, err := c.Connect().CARoots(nil)
	require.NoError(t, err)
	return list
}

func activeRoot(t retry.TestingTB, list *api.CARootList) *api.CARoot {
	for _, r := range list.Roots {
		if r.Active {
			return r
		}
	}
	require.FailNow(t, "no active root")
	return nil
}

// chain parses a PEM bundle into certificates.
func chain(t retry.TestingTB, bundle string) []*x509.Certificate {
	var certs []*x509.Certificate
	rest := []byte(bundle)
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		certs = append(certs, cert)
	}
	require.NotEmpty(t, certs)
	return certs
}

// validatesAgainst reports whether the leaf in leafPEM chains to one of the given
// roots, using the intermediates delivered with it.
func validatesAgainst(t retry.TestingTB, leafPEM string, rootPEMs ...string) error {
	certs := chain(t, leafPEM)

	pool := x509.NewCertPool()
	for _, r := range rootPEMs {
		require.True(t, pool.AppendCertsFromPEM([]byte(r)))
	}
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	_, err := certs[0].Verify(x509.VerifyOptions{Roots: pool, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	return err
}
