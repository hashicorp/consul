// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package wanfed

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func setEntry(t *testing.T, c *api.Client, entry api.ConfigEntry) {
	t.Helper()
	ok, _, err := c.ConfigEntries().Set(entry, nil)
	require.NoError(t, err)
	require.True(t, ok)
}

// entryExists reports whether the entry is readable from c's datacenter.
func entryExists(t retry.TestingTB, c *api.Client, kind, name string) bool {
	entry, _, err := c.ConfigEntries().Get(kind, name, nil)
	if err != nil {
		require.Contains(t, err.Error(), "404", "unexpected error reading %s/%s", kind, name)
		return false
	}
	return entry != nil
}

// TestWanFed_ConfigEntryReplication verifies that config entries are
// replicated from the primary datacenter to the secondary, that writes made in
// the secondary are forwarded to the primary, and that deletions replicate.
func TestWanFed_ConfigEntryReplication(t *testing.T) {
	t.Parallel()

	f := newFederation(t)

	t.Run("entries written in the primary appear in the secondary", func(t *testing.T) {
		setEntry(t, f.pc, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "web", Protocol: "http"})
		setEntry(t, f.pc, &api.ProxyConfigEntry{Kind: api.ProxyDefaults, Name: api.ProxyConfigGlobal, Config: map[string]any{"protocol": "http"}})
		setEntry(t, f.pc, &api.MeshConfigEntry{TransparentProxy: api.TransparentProxyMeshConfig{MeshDestinationsOnly: true}})
		setEntry(t, f.pc, &api.ServiceResolverConfigEntry{Kind: api.ServiceResolver, Name: "web", DefaultSubset: "v1", Subsets: map[string]api.ServiceResolverSubset{"v1": {Filter: "Service.Meta.version == v1"}}})

		eventually(t, func(r *retry.R) {
			sd, _, err := f.sc.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
			require.NoError(r, err)
			require.Equal(r, "http", sd.(*api.ServiceConfigEntry).Protocol)

			pd, _, err := f.sc.ConfigEntries().Get(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
			require.NoError(r, err)
			require.Equal(r, "http", pd.(*api.ProxyConfigEntry).Config["protocol"])

			mesh, _, err := f.sc.ConfigEntries().Get(api.MeshConfig, api.MeshConfigMesh, nil)
			require.NoError(r, err)
			require.True(r, mesh.(*api.MeshConfigEntry).TransparentProxy.MeshDestinationsOnly)

			res, _, err := f.sc.ConfigEntries().Get(api.ServiceResolver, "web", nil)
			require.NoError(r, err)
			require.Equal(r, "v1", res.(*api.ServiceResolverConfigEntry).DefaultSubset)
		})
	})

	t.Run("updates replicate", func(t *testing.T) {
		setEntry(t, f.pc, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "web", Protocol: "grpc", Meta: map[string]string{"rev": "2"}})

		eventually(t, func(r *retry.R) {
			sd, _, err := f.sc.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
			require.NoError(r, err)
			got := sd.(*api.ServiceConfigEntry)
			require.Equal(r, "grpc", got.Protocol)
			require.Equal(r, "2", got.Meta["rev"])
		})
	})

	t.Run("a write made in the secondary is forwarded to the primary", func(t *testing.T) {
		setEntry(t, f.sc, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "from-secondary", Protocol: "tcp"})

		eventually(t, func(r *retry.R) {
			require.True(r, entryExists(r, f.pc, api.ServiceDefaults, "from-secondary"), "the primary must own the write")
			require.True(r, entryExists(r, f.sc, api.ServiceDefaults, "from-secondary"), "and it replicates back")
		})
	})

	t.Run("deletes replicate", func(t *testing.T) {
		_, err := f.pc.ConfigEntries().Delete(api.ServiceDefaults, "web", nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			require.False(r, entryExists(r, f.sc, api.ServiceDefaults, "web"))
		})

		_, err = f.sc.ConfigEntries().Delete(api.ServiceDefaults, "from-secondary", nil)
		require.NoError(t, err)
		eventually(t, func(r *retry.R) {
			require.False(r, entryExists(r, f.pc, api.ServiceDefaults, "from-secondary"))
			require.False(r, entryExists(r, f.sc, api.ServiceDefaults, "from-secondary"))
		})
	})
}

// TestWanFed_IntentionReplication verifies that intentions defined in the
// primary datacenter are enforced consistently in the secondary.
func TestWanFed_IntentionReplication(t *testing.T) {
	t.Parallel()

	f := newFederation(t)

	allowed := func(r retry.TestingTB, c *api.Client, source, destination string) bool {
		ok, _, err := c.Connect().IntentionCheck(&api.IntentionCheck{Source: source, Destination: destination}, nil)
		require.NoError(r, err)
		return ok
	}

	t.Run("a deny created in the primary applies in the secondary", func(t *testing.T) {
		require.True(t, allowed(t, f.sc, "web", "db"), "default policy allows before any intention")

		setEntry(t, f.pc, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "db",
			Sources: []*api.SourceIntention{{Name: "web", Action: api.IntentionActionDeny}},
		})

		eventually(t, func(r *retry.R) {
			require.False(r, allowed(r, f.sc, "web", "db"))
			require.False(r, allowed(r, f.pc, "web", "db"))

			ixn, _, err := f.sc.Connect().IntentionGetExact("web", "db", nil)
			require.NoError(r, err)
			require.NotNil(r, ixn)
			require.Equal(r, api.IntentionActionDeny, ixn.Action)
		})
	})

	t.Run("changing it to allow flips the decision in both datacenters", func(t *testing.T) {
		setEntry(t, f.pc, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "db",
			Sources: []*api.SourceIntention{{Name: "web", Action: api.IntentionActionAllow}},
		})

		eventually(t, func(r *retry.R) {
			require.True(r, allowed(r, f.sc, "web", "db"))
			require.True(r, allowed(r, f.pc, "web", "db"))
		})
	})

	t.Run("a wildcard deny replicates and exact intentions still take precedence", func(t *testing.T) {
		setEntry(t, f.pc, &api.ServiceIntentionsConfigEntry{
			Kind:    api.ServiceIntentions,
			Name:    "*",
			Sources: []*api.SourceIntention{{Name: "*", Action: api.IntentionActionDeny}},
		})

		eventually(t, func(r *retry.R) {
			require.False(r, allowed(r, f.sc, "anything", "unrelated"))
			require.True(r, allowed(r, f.sc, "web", "db"), "the exact web->db allow wins over *->* deny")
		})
	})

	t.Run("removing the intentions restores the default", func(t *testing.T) {
		for _, name := range []string{"db", "*"} {
			_, err := f.pc.ConfigEntries().Delete(api.ServiceIntentions, name, nil)
			require.NoError(t, err)
		}

		eventually(t, func(r *retry.R) {
			require.True(r, allowed(r, f.sc, "anything", "unrelated"))
		})
	})
}

func rootNames(t retry.TestingTB, c *api.Client) (active string, trustDomain string, roots []*api.CARoot) {
	list, _, err := c.Connect().CARoots(nil)
	require.NoError(t, err)
	return list.ActiveRootID, list.TrustDomain, list.Roots
}

// verifyLeaf checks that a leaf certificate issued for service in datacenter dc
// chains to one of the trusted roots through the intermediates it carries.
func verifyLeaf(t retry.TestingTB, leaf *api.LeafCert, roots []*api.CARoot, dc string) (chainLength int) {
	rootPool := x509.NewCertPool()
	for _, r := range roots {
		require.True(t, rootPool.AppendCertsFromPEM([]byte(r.RootCertPEM)), "root %s is not valid PEM", r.ID)
	}

	var certs []*x509.Certificate
	rest := []byte(leaf.CertPEM)
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

	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}
	_, err := certs[0].Verify(x509.VerifyOptions{Roots: rootPool, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny}})
	require.NoError(t, err, "the leaf must chain to a trusted root")

	require.Len(t, certs[0].URIs, 1)
	require.Contains(t, certs[0].URIs[0].Path, "/dc/"+dc+"/")
	return len(certs)
}

// TestWanFed_ConnectCAReplication verifies the secondary datacenter shares the
// primary's trust domain and root, signs workload certificates through its own
// intermediate, and follows a root rotation in the primary.
func TestWanFed_ConnectCAReplication(t *testing.T) {
	t.Parallel()

	f := newFederation(t)

	require.NoError(t, f.sc.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: "web", Port: 8080}))

	t.Run("both datacenters trust the same root", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			primaryActive, primaryDomain, primaryRoots := rootNames(r, f.pc)
			secondaryActive, secondaryDomain, secondaryRoots := rootNames(r, f.sc)

			require.NotEmpty(r, primaryDomain)
			require.Equal(r, primaryDomain, secondaryDomain, "one trust domain per federation")
			require.Equal(r, primaryActive, secondaryActive)
			require.Len(r, primaryRoots, 1)
			require.Len(r, secondaryRoots, 1)
		})
	})

	t.Run("the secondary issues certificates that chain to the primary's root", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			leaf, _, err := f.sc.Agent().ConnectCALeaf("web", nil)
			require.NoError(r, err)
			_, _, roots := rootNames(r, f.sc)
			chain := verifyLeaf(r, leaf, roots, secondaryDC)
			require.GreaterOrEqual(r, chain, 2, "the secondary signs with an intermediate issued by the primary")
		})
	})

	t.Run("a root rotation in the primary reaches the secondary", func(t *testing.T) {
		before, _, _ := rootNames(t, f.pc)
		rotateCARoot(t, f.pc)

		eventually(t, func(r *retry.R) {
			primaryActive, _, primaryRoots := rootNames(r, f.pc)
			secondaryActive, _, secondaryRoots := rootNames(r, f.sc)

			require.Len(r, primaryRoots, 2, "the old root stays trusted while the new one is rolled out")
			require.Len(r, secondaryRoots, 2)
			require.NotEqual(r, before, primaryActive, "a new root became active")
			require.Equal(r, primaryActive, secondaryActive)
		})

		eventually(t, func(r *retry.R) {
			leaf, _, err := f.sc.Agent().ConnectCALeaf("web", nil)
			require.NoError(r, err)
			_, _, roots := rootNames(r, f.sc)
			verifyLeaf(r, leaf, roots, secondaryDC)
		})
	})
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
