// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connectca

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

func caConfig(t *testing.T, c *api.Client) *api.CAConfig {
	t.Helper()
	conf, _, err := c.Connect().CAGetConfig(nil)
	require.NoError(t, err)
	return conf
}

func leaf(t retry.TestingTB, c *api.Client) *api.LeafCert {
	cert, _, err := c.Agent().ConnectCALeaf("web", nil)
	require.NoError(t, err)
	return cert
}

// TestConnectCA_BuiltinConfiguration verifies the built-in provider's defaults,
// how its settings change the certificates it issues, and that invalid
// settings are refused without changing anything.
func TestConnectCA_BuiltinConfiguration(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	t.Run("the defaults", func(t *testing.T) {
		conf := caConfig(t, e.server)

		require.Equal(t, "consul", conf.Provider)
		require.Equal(t, "72h", conf.Config["LeafCertTTL"])
		require.Equal(t, "8760h", conf.Config["IntermediateCertTTL"])
		require.Equal(t, "87600h", conf.Config["RootCertTTL"])
		require.False(t, conf.ForceWithoutCrossSigning)
	})

	t.Run("the trust bundle has one active root and a trust domain", func(t *testing.T) {
		list := roots(t, e.server)

		require.Regexp(t, `^[0-9a-f-]{36}\.consul$`, list.TrustDomain)
		require.Len(t, list.Roots, 1)
		root := activeRoot(t, list)
		require.Equal(t, list.ActiveRootID, root.ID)
		require.Equal(t, "Consul CA Primary Cert", root.Name)

		clientView, _, err := e.client.Agent().ConnectCARoots(nil)
		require.NoError(t, err)
		require.Equal(t, list.ActiveRootID, clientView.ActiveRootID, "agents serve the same bundle as the servers")
	})

	t.Run("a leaf certificate carries the service's SPIFFE identity and chains to the root", func(t *testing.T) {
		cert := leaf(t, e.client)
		list := roots(t, e.server)

		require.Equal(t, "web", cert.Service)
		require.Equal(t, "spiffe://"+list.TrustDomain+"/ns/default/dc/dc1/svc/web", cert.ServiceURI)

		certs := chain(t, cert.CertPEM)
		require.Len(t, certs[0].URIs, 1)
		require.Equal(t, cert.ServiceURI, certs[0].URIs[0].String())
		require.NoError(t, validatesAgainst(t, cert.CertPEM, activeRoot(t, list).RootCertPEM))
	})

	t.Run("the leaf lifetime follows LeafCertTTL", func(t *testing.T) {
		conf := caConfig(t, e.server)
		conf.Config["LeafCertTTL"] = "2h"
		_, err := e.server.Connect().CASetConfig(conf, nil)
		require.NoError(t, err)

		// Certificates are cached until they need renewal, so a service that has not
		// asked for one yet shows the new lifetime.
		require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{Name: "late-service", Port: 8081}))
		eventually(t, func(r *retry.R) {
			cert, _, err := e.client.Agent().ConnectCALeaf("late-service", nil)
			require.NoError(r, err)
			lifetime := cert.ValidBefore.Sub(cert.ValidAfter)
			require.InDelta(r, (2 * time.Hour).Seconds(), lifetime.Seconds(), 60, "leaf lifetime is %s", lifetime)
		})
	})

	t.Run("invalid settings are refused and leave the configuration alone", func(t *testing.T) {
		before := caConfig(t, e.server)

		for name, tc := range map[string]struct {
			config  map[string]interface{}
			wantErr string
		}{
			"a leaf TTL longer than the intermediate": {map[string]interface{}{"LeafCertTTL": "100000h", "IntermediateCertTTL": "8760h"}, "leaf cert TTL must be less than"},
			"a leaf TTL under an hour":                {map[string]interface{}{"LeafCertTTL": "10s"}, "leaf cert TTL must be greater or equal than 1h0m0s"},
			"an unsupported key type":                 {map[string]interface{}{"PrivateKeyType": "dsa", "PrivateKeyBits": 2048}, "private key type must be either 'ec' or 'rsa'"},
			"an unsupported RSA key length":           {map[string]interface{}{"PrivateKeyType": "rsa", "PrivateKeyBits": 1000}, "RSA key length must be 2048 or 4096 bits"},
		} {
			_, err := e.server.Connect().CASetConfig(&api.CAConfig{Provider: "consul", Config: tc.config}, nil)
			require.Error(t, err, name)
			require.Contains(t, err.Error(), tc.wantErr, name)
		}

		_, err := e.server.Connect().CASetConfig(&api.CAConfig{Provider: "no-such-provider", Config: map[string]interface{}{}}, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), `unknown CA provider "no-such-provider"`)

		after := caConfig(t, e.server)
		require.Equal(t, before.Provider, after.Provider)
		require.Equal(t, before.Config, after.Config)
		require.Equal(t, before.ModifyIndex, after.ModifyIndex, "a refused change must not touch the stored configuration")
	})
}

// TestConnectCA_RootRotation verifies what changing the CA's key type does:
// a new root becomes active, the old one stays trusted, and, unless told not
// to, certificates from the new root carry a cross-signature so that peers that
// only know the old root still trust them.
func TestConnectCA_RootRotation(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	original := activeRoot(t, roots(t, e.server))
	before := leaf(t, e.client)
	require.NoError(t, validatesAgainst(t, before.CertPEM, original.RootCertPEM))

	t.Run("changing the key type creates and activates a new root", func(t *testing.T) {
		conf := caConfig(t, e.server)
		conf.Config["PrivateKeyType"] = "rsa"
		conf.Config["PrivateKeyBits"] = 2048
		_, err := e.server.Connect().CASetConfig(conf, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			list := roots(r, e.server)
			require.Len(r, list.Roots, 2)
			require.NotEqual(r, original.ID, list.ActiveRootID)
		})

		list := roots(t, e.server)
		var old *api.CARoot
		for _, root := range list.Roots {
			if root.ID == original.ID {
				old = root
			}
		}
		require.NotNil(t, old, "the previous root stays in the bundle")
		require.False(t, old.Active)
	})

	t.Run("a certificate from the new root is trusted by clients that only know either root", func(t *testing.T) {
		eventually(t, func(r *retry.R) {
			list := roots(r, e.server)
			current := activeRoot(r, list)
			cert := leaf(r, e.client)

			require.NotEqual(r, before.SerialNumber, cert.SerialNumber, "a new certificate was issued by the new root")
			require.NoError(r, validatesAgainst(r, cert.CertPEM, current.RootCertPEM), "chains to the new root")
			require.NoError(r, validatesAgainst(r, cert.CertPEM, original.RootCertPEM), "and, through the cross-signature, to the old root")
		})
	})

	t.Run("certificates issued before the rotation stay valid under the old root", func(t *testing.T) {
		require.NoError(t, validatesAgainst(t, before.CertPEM, original.RootCertPEM))
	})

	t.Run("rotating without cross-signing leaves old-root-only clients unable to trust new certificates", func(t *testing.T) {
		conf := caConfig(t, e.server)
		conf.Config["PrivateKeyType"] = "ec"
		conf.Config["PrivateKeyBits"] = 256
		conf.ForceWithoutCrossSigning = true
		_, err := e.server.Connect().CASetConfig(conf, nil)
		require.NoError(t, err)

		eventually(t, func(r *retry.R) {
			list := roots(r, e.server)
			require.Len(r, list.Roots, 3)
			current := activeRoot(r, list)
			cert := leaf(r, e.client)

			require.NoError(r, validatesAgainst(r, cert.CertPEM, current.RootCertPEM))
			require.Error(r, validatesAgainst(r, cert.CertPEM, original.RootCertPEM), "no cross-signature links the new root to the original one")
		})
	})
}

// TestConnectCA_CLI covers `consul connect ca`.
func TestConnectCA_CLI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	t.Run("get-config prints the provider settings", func(t *testing.T) {
		res := e.shell(t, "consul connect ca get-config")
		require.Equal(t, 0, res.ExitCode, res.Output)

		var conf api.CAConfig
		require.NoError(t, json.Unmarshal([]byte(res.Output), &conf), res.Output)
		require.Equal(t, "consul", conf.Provider)
		require.Equal(t, "72h", conf.Config["LeafCertTTL"])
	})

	t.Run("set-config applies a file", func(t *testing.T) {
		res := e.shell(t, `printf '%s' '{"Provider": "consul", "Config": {"LeafCertTTL": "48h"}}' > /tmp/ca.json && consul connect ca set-config -config-file /tmp/ca.json`)
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, "Configuration updated!")

		require.Equal(t, "48h", caConfig(t, e.server).Config["LeafCertTTL"])
	})

	t.Run("set-config reports a rejected configuration", func(t *testing.T) {
		res := e.shell(t, `printf '%s' '{"Provider": "consul", "Config": {"PrivateKeyType": "dsa", "PrivateKeyBits": 2048}}' > /tmp/bad-ca.json && consul connect ca set-config -config-file /tmp/bad-ca.json`)

		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "private key type must be either")
	})
}
