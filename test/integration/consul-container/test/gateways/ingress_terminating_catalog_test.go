// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// linkedServices indexes the gateway-services view of a gateway by service name.
func linkedServices(t retry.TestingTB, c *api.Client, gateway string) map[string]*api.GatewayService {
	svcs, _, err := c.Catalog().GatewayServices(gateway, nil)
	require.NoError(t, err)

	out := map[string]*api.GatewayService{}
	for _, s := range svcs {
		out[s.Service.Name] = s
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestIngressGateway_ConfigEntryAndCatalog verifies how an ingress-gateway
// config entry is reflected in the catalog: which services a gateway serves,
// on which port and hostnames, and which gateways front a service. These views
// are what proxies, DNS and the UI are built from.
func TestIngressGateway_ConfigEntryAndCatalog(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)
	c := e.server

	for _, svc := range []string{"web", "api"} {
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: svc, Protocol: "http"})
	}

	t.Run("services are linked to the gateway with listener details", func(t *testing.T) {
		setEntry(t, c, &api.IngressGatewayConfigEntry{
			Kind: api.IngressGateway,
			Name: "ingress",
			Listeners: []api.IngressListener{{
				Port:     8080,
				Protocol: "http",
				Services: []api.IngressService{
					{Name: "web", Hosts: []string{"web.example.com"}},
					{Name: "api"},
				},
			}},
		})

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			linked := linkedServices(r, c, "ingress")
			require.Equal(r, []string{"api", "web"}, sortedKeys(linked))

			web := linked["web"]
			require.Equal(r, api.ServiceKindIngressGateway, web.GatewayKind)
			require.Equal(r, "ingress", web.Gateway.Name)
			require.Equal(r, 8080, web.Port)
			require.Equal(r, "http", web.Protocol)
			require.Equal(r, []string{"web.example.com"}, web.Hosts)
			require.Empty(r, linked["api"].Hosts)
		})
	})

	t.Run("a registered gateway instance is discoverable per service", func(t *testing.T) {
		require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
			Kind:    api.ServiceKindIngressGateway,
			ID:      "ingress-1",
			Name:    "ingress",
			Address: "10.9.9.9",
			Port:    8080,
		}))

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			for _, svc := range []string{"web", "api"} {
				entries, _, err := c.Health().Ingress(svc, false, nil)
				require.NoError(r, err)
				require.Len(r, entries, 1, svc)
				require.Equal(r, "ingress", entries[0].Service.Service)
				require.Equal(r, "10.9.9.9", entries[0].Service.Address)
			}

			// A service that is not behind the gateway has no ingress.
			entries, _, err := c.Health().Ingress("not-exposed", false, nil)
			require.NoError(r, err)
			require.Empty(r, entries)
		})
	})

	t.Run("removing a service from the entry unlinks it", func(t *testing.T) {
		setEntry(t, c, &api.IngressGatewayConfigEntry{
			Kind: api.IngressGateway,
			Name: "ingress",
			Listeners: []api.IngressListener{{
				Port: 8080, Protocol: "http", Services: []api.IngressService{{Name: "web"}},
			}},
		})

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, []string{"web"}, sortedKeys(linkedServices(r, c, "ingress")))
			require.Empty(r, linkedServices(r, c, "ingress")["web"].Hosts, "hosts removed from the entry must be dropped")

			entries, _, err := c.Health().Ingress("api", false, nil)
			require.NoError(r, err)
			require.Empty(r, entries)
		})
	})

	t.Run("multiple listeners link their own services", func(t *testing.T) {
		setEntry(t, c, &api.IngressGatewayConfigEntry{
			Kind: api.IngressGateway,
			Name: "ingress",
			Listeners: []api.IngressListener{
				{Port: 8080, Protocol: "http", Services: []api.IngressService{{Name: "web"}}},
				{Port: 8081, Protocol: "http", Services: []api.IngressService{{Name: "api"}}},
			},
		})

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			linked := linkedServices(r, c, "ingress")
			require.Equal(r, []string{"api", "web"}, sortedKeys(linked))
			require.Equal(r, 8080, linked["web"].Port)
			require.Equal(r, 8081, linked["api"].Port)
		})
	})

	t.Run("deleting the entry removes every link", func(t *testing.T) {
		_, err := c.ConfigEntries().Delete(api.IngressGateway, "ingress", nil)
		require.NoError(t, err)

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Empty(r, linkedServices(r, c, "ingress"))

			entries, _, err := c.Health().Ingress("web", false, nil)
			require.NoError(r, err)
			require.Empty(r, entries)
		})
	})
}

// TestIngressGateway_ConfigEntryValidation verifies the listener rules that
// servers enforce when an ingress-gateway entry is written.
func TestIngressGateway_ConfigEntryValidation(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)

	ingress := func(name string, tls *api.GatewayTLSConfig, listeners ...api.IngressListener) *api.IngressGatewayConfigEntry {
		entry := &api.IngressGatewayConfigEntry{Kind: api.IngressGateway, Name: name, Listeners: listeners}
		if tls != nil {
			entry.TLS = *tls
		}
		return entry
	}

	for name, tc := range map[string]struct {
		entry   *api.IngressGatewayConfigEntry
		wantErr string
	}{
		"the same port on two listeners": {
			entry: ingress("dup-port", nil,
				api.IngressListener{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "a"}}},
				api.IngressListener{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "b"}}},
			),
			wantErr: "port 8080 declared on two listeners",
		},
		"a wildcard service on a tcp listener": {
			entry: ingress("tcp-wildcard", nil,
				api.IngressListener{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "*"}}}),
			wantErr: "Wildcard service name is only valid for protocol = 'http'",
		},
		"hostnames on a tcp listener": {
			entry: ingress("tcp-hosts", nil,
				api.IngressListener{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "a", Hosts: []string{"a.example.com"}}}}),
			wantErr: "Associating hosts to a service is not supported for the tcp protocol",
		},
		"hostnames on a wildcard service": {
			entry: ingress("wildcard-hosts", nil,
				api.IngressListener{Port: 8080, Protocol: "http", Services: []api.IngressService{{Name: "*", Hosts: []string{"x.example.com"}}}}),
			wantErr: "Associating hosts to a wildcard service is not supported",
		},
		"an unsupported protocol": {
			entry: ingress("udp", nil,
				api.IngressListener{Port: 8080, Protocol: "udp", Services: []api.IngressService{{Name: "a"}}}),
			wantErr: "protocol must be 'tcp', 'http', 'http2', or 'grpc'",
		},
		"an unknown tls version": {
			entry: ingress("bad-tls", &api.GatewayTLSConfig{Enabled: true, TLSMinVersion: "TLSv9"},
				api.IngressListener{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "a"}}}),
			wantErr: "no matching TLS version found for TLSv9",
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			requireSetFails(t, e.server, tc.entry, tc.wantErr)
		})
	}

	t.Run("valid tls settings are stored", func(t *testing.T) {
		entry := ingress("good-tls",
			&api.GatewayTLSConfig{Enabled: true, TLSMinVersion: "TLSv1_2", TLSMaxVersion: "TLSv1_3"},
			api.IngressListener{Port: 8080, Protocol: "tcp", Services: []api.IngressService{{Name: "a"}}})
		setEntry(t, e.server, entry)

		got, _, err := e.server.ConfigEntries().Get(api.IngressGateway, "good-tls", nil)
		require.NoError(t, err)
		tls := got.(*api.IngressGatewayConfigEntry).TLS
		require.True(t, tls.Enabled)
		require.Equal(t, "TLSv1_2", tls.TLSMinVersion)
		require.Equal(t, "TLSv1_3", tls.TLSMaxVersion)
	})
}

// TestTerminatingGateway_ConfigEntryAndCatalog verifies the terminating
// gateway view: linked services keep their TLS origination settings, edits
// replace them, and invalid TLS origination is rejected.
func TestTerminatingGateway_ConfigEntryAndCatalog(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)
	c := e.server

	t.Run("linked services carry tls origination settings", func(t *testing.T) {
		setEntry(t, c, &api.TerminatingGatewayConfigEntry{
			Kind: api.TerminatingGateway,
			Name: "terminating",
			Services: []api.LinkedService{
				{Name: "legacy-db", CAFile: "/certs/ca.pem", CertFile: "/certs/cert.pem", KeyFile: "/certs/key.pem", SNI: "db.example.com"},
				{Name: "legacy-cache"},
			},
		})

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			linked := linkedServices(r, c, "terminating")
			require.Equal(r, []string{"legacy-cache", "legacy-db"}, sortedKeys(linked))

			db := linked["legacy-db"]
			require.Equal(r, api.ServiceKindTerminatingGateway, db.GatewayKind)
			require.Equal(r, "/certs/ca.pem", db.CAFile)
			require.Equal(r, "/certs/cert.pem", db.CertFile)
			require.Equal(r, "/certs/key.pem", db.KeyFile)
			require.Equal(r, "db.example.com", db.SNI)

			cache := linked["legacy-cache"]
			require.Empty(r, cache.CAFile)
			require.Empty(r, cache.SNI)
		})
	})

	t.Run("editing the entry replaces the linked services", func(t *testing.T) {
		setEntry(t, c, &api.TerminatingGatewayConfigEntry{
			Kind:     api.TerminatingGateway,
			Name:     "terminating",
			Services: []api.LinkedService{{Name: "legacy-cache"}, {Name: "legacy-queue", SNI: "queue.example.com"}},
		})

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			linked := linkedServices(r, c, "terminating")
			require.Equal(r, []string{"legacy-cache", "legacy-queue"}, sortedKeys(linked))
			require.Equal(r, "queue.example.com", linked["legacy-queue"].SNI)
		})
	})

	t.Run("a service can be linked to more than one terminating gateway", func(t *testing.T) {
		setEntry(t, c, &api.TerminatingGatewayConfigEntry{
			Kind:     api.TerminatingGateway,
			Name:     "terminating-b",
			Services: []api.LinkedService{{Name: "legacy-cache"}},
		})

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Equal(r, []string{"legacy-cache"}, sortedKeys(linkedServices(r, c, "terminating-b")))
			require.Contains(r, linkedServices(r, c, "terminating"), "legacy-cache")
		})
	})

	t.Run("incomplete tls origination is rejected", func(t *testing.T) {
		requireSetFails(t, c, &api.TerminatingGatewayConfigEntry{
			Kind:     api.TerminatingGateway,
			Name:     "bad-terminating",
			Services: []api.LinkedService{{Name: "x", CertFile: "/certs/cert.pem"}},
		}, "must have a CertFile, CAFile, and KeyFile specified for TLS origination")
	})

	t.Run("deleting the entry removes every link", func(t *testing.T) {
		_, err := c.ConfigEntries().Delete(api.TerminatingGateway, "terminating", nil)
		require.NoError(t, err)

		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			require.Empty(r, linkedServices(r, c, "terminating"))
			require.NotEmpty(r, linkedServices(r, c, "terminating-b"), "other gateways are unaffected")
		})
	})
}
