// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
)

func ingressListener(t *testing.T, c *api.Client, gateway string, port int) *api.IngressListener {
	t.Helper()

	entry, _, err := c.ConfigEntries().Get(api.IngressGateway, gateway, nil)
	require.NoError(t, err)
	for _, l := range entry.(*api.IngressGatewayConfigEntry).Listeners {
		if l.Port == port {
			l := l
			return &l
		}
	}
	return nil
}

// TestConnectExpose verifies `consul connect expose`, which exposes a mesh
// service through an ingress gateway by writing the ingress-gateway config
// entry and an allow intention from the gateway to the service.
func TestConnectExpose(t *testing.T) {
	t.Parallel()

	e := newEnv(t)

	t.Run("tcp service gets a listener and an allow intention", func(t *testing.T) {
		res := e.cliCombined(t, "connect", "expose", "-service", "tcp-backend", "-ingress-gateway", "ingress", "-port", "8888", "-protocol", "tcp")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, `Successfully updated config entry for ingress service "ingress"`)
		require.Contains(t, res.Output, `Successfully set up intention for "ingress" -> "tcp-backend"`)

		l := ingressListener(t, e.server, "ingress", 8888)
		require.NotNil(t, l)
		require.Equal(t, "tcp", l.Protocol)
		require.Len(t, l.Services, 1)
		require.Equal(t, "tcp-backend", l.Services[0].Name)

		ixn, _, err := e.server.Connect().IntentionGetExact("ingress", "tcp-backend", nil)
		require.NoError(t, err)
		require.NotNil(t, ixn)
		require.Equal(t, api.IntentionActionAllow, ixn.Action)
	})

	t.Run("exposing the same service again is idempotent", func(t *testing.T) {
		res := e.cliCombined(t, "connect", "expose", "-service", "tcp-backend", "-ingress-gateway", "ingress", "-port", "8888", "-protocol", "tcp")
		require.Equal(t, 0, res.ExitCode, res.Output)
		require.Contains(t, res.Output, `Intention already exists for "ingress" -> "tcp-backend"`)

		l := ingressListener(t, e.server, "ingress", 8888)
		require.NotNil(t, l)
		require.Len(t, l.Services, 1)
	})

	t.Run("a listener cannot be shared across protocols", func(t *testing.T) {
		res := e.cliCombined(t, "connect", "expose", "-service", "tcp-backend", "-ingress-gateway", "ingress", "-port", "8888", "-protocol", "http")
		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, `Listener on port 8888 already configured with conflicting protocol "tcp"`)
	})

	t.Run("http service with hostnames", func(t *testing.T) {
		ok, _, err := e.server.ConfigEntries().Set(&api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "http-backend", Protocol: "http"}, nil)
		require.NoError(t, err)
		require.True(t, ok)

		res := e.cliCombined(t, "connect", "expose", "-service", "http-backend", "-ingress-gateway", "ingress", "-port", "8889", "-protocol", "http", "-host", "app.example.com")
		require.Equal(t, 0, res.ExitCode, res.Output)

		l := ingressListener(t, e.server, "ingress", 8889)
		require.NotNil(t, l)
		require.Equal(t, "http", l.Protocol)
		require.Len(t, l.Services, 1)
		require.Equal(t, "http-backend", l.Services[0].Name)
		require.Contains(t, l.Services[0].Hosts, "app.example.com")
	})

	t.Run("service protocol must match the listener protocol", func(t *testing.T) {
		// No service-defaults: the service is tcp, so it cannot sit behind an http listener.
		res := e.cliCombined(t, "connect", "expose", "-service", "untyped-backend", "-ingress-gateway", "ingress", "-port", "8890", "-protocol", "http")
		require.NotEqual(t, 0, res.ExitCode)
		require.Contains(t, res.Output, "does not match defined listener protocol")
		require.Nil(t, ingressListener(t, e.server, "ingress", 8890))
	})
}
