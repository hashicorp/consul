// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connectca

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
	libutils "github.com/hashicorp/consul/test/integration/consul-container/libs/utils"
)

const (
	vaultImage     = libservice.HashicorpDockerProxy + "/hashicorp/vault:1.15"
	vaultRootToken = "root"
	// vaultAddress is where Consul reaches Vault. The Vault container shares the
	// server agent's network namespace.
	vaultAddress = "http://127.0.0.1:8200"
)

// startVault runs Vault in dev mode next to the server agent.
func startVault(t *testing.T, e *env) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:      vaultImage,
		Name:       libutils.RandName("vault-ca"),
		Cmd:        []string{"server", "-dev", "-dev-listen-address=0.0.0.0:8200", "-dev-root-token-id=" + vaultRootToken},
		WaitingFor: wait.ForLog("Vault server started!").WithStartupTimeout(2 * time.Minute),
		AutoRemove: false,
	}
	info, err := libcluster.LaunchContainerOnNode(context.Background(), e.cluster.Agents[0], req, []string{"8200"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = info.Container.Terminate(context.Background()) })
}

// TestConnectCA_VaultProvider verifies switching the datacenter's CA from the
// built-in provider to Vault and back: the new provider issues a working root,
// certificates keep chaining to a trusted root throughout, and the switch is
// cross-signed so existing trust is not broken.
//
// It needs the Vault image, so it only runs when CONSUL_TEST_VAULT is set, as
// the nightly integration job does.
func TestConnectCA_VaultProvider(t *testing.T) {
	if os.Getenv("CONSUL_TEST_VAULT") == "" {
		t.Skip("set CONSUL_TEST_VAULT=1 to run; it needs the Vault container image")
	}

	t.Parallel()

	e := newEnv(t)
	startVault(t, e)

	original := activeRoot(t, roots(t, e.server))
	before := leaf(t, e.client)
	require.NoError(t, validatesAgainst(t, before.CertPEM, original.RootCertPEM))

	t.Run("switching to Vault activates a Vault-issued root", func(t *testing.T) {
		_, err := e.server.Connect().CASetConfig(&api.CAConfig{
			Provider: "vault",
			Config: map[string]interface{}{
				"Address":             vaultAddress,
				"Token":               vaultRootToken,
				"RootPKIPath":         "pki-root",
				"IntermediatePKIPath": "pki-intermediate",
			},
		}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			list := roots(r, e.server)
			require.NotEqual(r, original.ID, list.ActiveRootID)
			require.Len(r, list.Roots, 2, "the built-in root stays trusted")
			require.NotEqual(r, "Consul CA Primary Cert", activeRoot(r, list).Name)
		})

		conf := caConfig(t, e.server)
		require.Equal(t, "vault", conf.Provider)
	})

	t.Run("certificates from Vault chain to the new root and, through the cross-signature, to the old one", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			current := activeRoot(r, roots(r, e.server))
			cert := leaf(r, e.client)

			require.NotEqual(r, before.SerialNumber, cert.SerialNumber)
			require.NoError(r, validatesAgainst(r, cert.CertPEM, current.RootCertPEM))
			require.NoError(r, validatesAgainst(r, cert.CertPEM, original.RootCertPEM))
		})
	})

	t.Run("switching back to the built-in provider keeps issuing valid certificates", func(t *testing.T) {
		vaultRoot := activeRoot(t, roots(t, e.server))

		_, err := e.server.Connect().CASetConfig(&api.CAConfig{
			Provider: "consul",
			Config:   map[string]interface{}{"PrivateKeyType": "ec", "PrivateKeyBits": 256},
		}, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			list := roots(r, e.server)
			require.NotEqual(r, vaultRoot.ID, list.ActiveRootID)

			cert := leaf(r, e.client)
			require.NoError(r, validatesAgainst(r, cert.CertPEM, activeRoot(r, list).RootCertPEM))
		})
	})
}
