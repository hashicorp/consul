// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package dataplane

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/proto-public/pbconnectca"
	"github.com/hashicorp/consul/proto-public/pbdataplane"
	"github.com/hashicorp/consul/proto-public/pbserverdiscovery"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

const numServers = 3

// TestDataplaneAPIs exercises the external gRPC services that consul-dataplane
// and other agentless workloads depend on: feature discovery, Envoy bootstrap
// parameters, Connect CA roots and signing, and server discovery. The services
// are queried directly on a server's gRPC port.
func TestDataplaneAPIs(t *testing.T) {
	t.Parallel()

	e := newEnv(t, numServers)
	ctx := context.Background()

	t.Run("supported dataplane features", func(t *testing.T) {
		resp, err := pbdataplane.NewDataplaneServiceClient(e.conn).
			GetSupportedDataplaneFeatures(ctx, &pbdataplane.GetSupportedDataplaneFeaturesRequest{})
		require.NoError(t, err)

		support := map[pbdataplane.DataplaneFeatures]bool{}
		for _, f := range resp.SupportedDataplaneFeatures {
			support[f.FeatureName] = f.Supported
		}
		require.True(t, support[pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_WATCH_SERVERS])
		require.True(t, support[pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_EDGE_CERTIFICATE_MANAGEMENT])
		require.True(t, support[pbdataplane.DataplaneFeatures_DATAPLANE_FEATURES_ENVOY_BOOTSTRAP_CONFIGURATION])
	})

	t.Run("envoy bootstrap parameters", func(t *testing.T) {
		require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			Name: "web",
			Port: 8080,
			Connect: &api.AgentServiceConnect{
				SidecarService: &api.AgentServiceRegistration{},
			},
		}))
		nodeName, err := e.client.Agent().NodeName()
		require.NoError(t, err)

		ok, _, err := e.server.ConfigEntries().Set(&api.ProxyConfigEntry{
			Kind:       api.ProxyDefaults,
			Name:       api.ProxyConfigGlobal,
			Config:     map[string]any{"protocol": "http"},
			AccessLogs: &api.AccessLogsConfig{Enabled: true},
		}, nil)
		require.NoError(t, err)
		require.True(t, ok)

		dp := pbdataplane.NewDataplaneServiceClient(e.conn)
		req := &pbdataplane.GetEnvoyBootstrapParamsRequest{
			NodeSpec: &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeName{NodeName: nodeName},
			ProxyId:  "web-sidecar-proxy",
		}

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			resp, err := dp.GetEnvoyBootstrapParams(ctx, req)
			require.NoError(r, err)

			require.Equal(r, "web", resp.Identity)
			require.Equal(r, "dc1", resp.Datacenter)
			require.Equal(r, nodeName, resp.NodeName)
			require.Equal(r, "http", resp.Config.GetFields()["protocol"].GetStringValue(), "proxy-defaults must be merged into the bootstrap config")
			require.NotEmpty(r, resp.AccessLogs, "access logs enabled in proxy-defaults must be returned")
		})

		t.Run("unknown proxy is NotFound", func(t *testing.T) {
			_, err := dp.GetEnvoyBootstrapParams(ctx, &pbdataplane.GetEnvoyBootstrapParamsRequest{
				NodeSpec: &pbdataplane.GetEnvoyBootstrapParamsRequest_NodeName{NodeName: nodeName},
				ProxyId:  "does-not-exist",
			})
			require.Error(t, err)
			require.Equal(t, codes.NotFound, status.Code(err))
		})
	})

	t.Run("connect CA roots and signing", func(t *testing.T) {
		ca := pbconnectca.NewConnectCAServiceClient(e.conn)

		stream, err := ca.WatchRoots(ctx, &pbconnectca.WatchRootsRequest{})
		require.NoError(t, err)

		first, err := stream.Recv()
		require.NoError(t, err)
		require.NotEmpty(t, first.TrustDomain)
		require.Len(t, first.Roots, 1)
		require.Equal(t, first.ActiveRootId, first.Roots[0].Id)
		require.NotEmpty(t, first.Roots[0].RootCert)

		t.Run("sign a workload certificate", func(t *testing.T) {
			spiffe := &url.URL{Scheme: "spiffe", Host: first.TrustDomain, Path: "/ns/default/dc/dc1/svc/web"}
			resp, err := ca.Sign(ctx, &pbconnectca.SignRequest{Csr: newCSR(t, spiffe)})
			require.NoError(t, err)

			block, _ := pem.Decode([]byte(resp.CertPem))
			require.NotNil(t, block, "response is not PEM")
			leaf, err := x509.ParseCertificate(block.Bytes)
			require.NoError(t, err)
			require.Len(t, leaf.URIs, 1)
			require.Equal(t, spiffe.String(), leaf.URIs[0].String())

			rootBlock, _ := pem.Decode([]byte(first.Roots[0].RootCert))
			require.NotNil(t, rootBlock)
			root, err := x509.ParseCertificate(rootBlock.Bytes)
			require.NoError(t, err)
			pool := x509.NewCertPool()
			pool.AddCert(root)
			_, err = leaf.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
			require.NoError(t, err, "leaf must chain to the watched root")
		})

		t.Run("sign rejects a SPIFFE ID in a foreign trust domain", func(t *testing.T) {
			foreign := &url.URL{Scheme: "spiffe", Host: "other.consul", Path: "/ns/default/dc/dc1/svc/web"}
			_, err := ca.Sign(ctx, &pbconnectca.SignRequest{Csr: newCSR(t, foreign)})
			require.Error(t, err)
		})

		t.Run("rotation is pushed to the watcher", func(t *testing.T) {
			rotateCARoot(t, e.server)

			rotated := make(chan *pbconnectca.WatchRootsResponse, 1)
			go func() {
				for {
					msg, err := stream.Recv()
					if err != nil {
						return
					}
					if msg.ActiveRootId != first.ActiveRootId {
						rotated <- msg
						return
					}
				}
			}()

			select {
			case msg := <-rotated:
				require.Len(t, msg.Roots, 2, "the old root stays trusted while it is rotated out")
				require.NotEqual(t, first.ActiveRootId, msg.ActiveRootId)
			case <-time.After(60 * time.Second):
				t.Fatal("WatchRoots did not report the rotated root")
			}
		})
	})

	t.Run("watch servers follows membership", func(t *testing.T) {
		sd := pbserverdiscovery.NewServerDiscoveryServiceClient(e.conn)

		watchCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		stream, err := sd.WatchServers(watchCtx, &pbserverdiscovery.WatchServersRequest{})
		require.NoError(t, err)

		first, err := stream.Recv()
		require.NoError(t, err)
		require.Len(t, first.Servers, numServers)
		for _, s := range first.Servers {
			require.NotEmpty(t, s.Id)
			require.NotEmpty(t, s.Address)
			require.NotEmpty(t, s.Version)
		}

		// Gracefully remove a server that is neither the one being queried nor the leader.
		leader, err := e.cluster.Leader()
		require.NoError(t, err)
		var victim libcluster.Agent
		for _, srv := range e.cluster.Servers() {
			if srv != e.cluster.Agents[0] && srv != leader {
				victim = srv
				break
			}
		}
		require.NotNil(t, victim)
		require.NoError(t, e.cluster.Remove(victim))

		updates := make(chan int, 1)
		go func() {
			for {
				msg, err := stream.Recv()
				if err != nil {
					return
				}
				if len(msg.Servers) == numServers-1 {
					updates <- len(msg.Servers)
					return
				}
			}
		}()
		select {
		case <-updates:
		case <-time.After(2 * time.Minute):
			t.Fatal("WatchServers did not report the removed server")
		}
	})
}

// newCSR creates a PEM encoded certificate signing request carrying uri as its
// only SAN.
func newCSR(t *testing.T, uri *url.URL) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "web"},
		URIs:    []*url.URL{uri},
	}, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
}

// rotateCARoot switches the built-in CA to a new private key, which makes the
// servers generate and activate a new root.
func rotateCARoot(t *testing.T, c *api.Client) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	conf, _, err := c.Connect().CAGetConfig(nil)
	require.NoError(t, err)
	conf.Config["PrivateKey"] = string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	conf.Config["PrivateKeyType"] = "ec"
	conf.Config["PrivateKeyBits"] = 256
	_, err = c.Connect().CASetConfig(conf, nil)
	require.NoError(t, err)
}
