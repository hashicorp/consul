// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
	libservice "github.com/hashicorp/consul/test/integration/consul-container/libs/service"
)

const (
	serverSidecarPort = 21100
	clientSidecarPort = 21101
)

// startBuiltinProxy runs `consul connect proxy <args>` in a container that
// shares the client agent's network namespace.
func (e *env) startBuiltinProxy(t *testing.T, name, serviceID string, bindPorts []int, args ...string) *libservice.ConnectContainer {
	t.Helper()

	proxy, err := libservice.NewConnectService(
		context.Background(),
		libservice.SidecarConfig{Name: name, ServiceID: serviceID},
		bindPorts,
		e.clientAgent(),
		func(req testcontainers.ContainerRequest) testcontainers.ContainerRequest {
			req.Cmd = append([]string{"consul", "connect", "proxy"}, args...)
			return req
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = proxy.Terminate() })
	return proxy
}

// httpGet returns the response body, or an error if the request failed at the
// transport level or did not return 200.
func httpGet(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return string(body), fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return string(body), nil
}

func requireReachable(t *testing.T, url string) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		_, err := httpGet(url)
		require.NoError(r, err)
	})
}

func requireBlocked(t *testing.T, url string) {
	t.Helper()
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		_, err := httpGet(url)
		require.Error(r, err, "connection should have been rejected by the destination proxy")
	})
}

// intentionEntry returns a service-intentions entry applying action to source -> destination.
func intentionEntry(destination, source string, action api.IntentionAction) *api.ServiceIntentionsConfigEntry {
	return &api.ServiceIntentionsConfigEntry{
		Kind:    api.ServiceIntentions,
		Name:    destination,
		Sources: []*api.SourceIntention{{Name: source, Action: action, Type: api.IntentionSourceConsul}},
	}
}

// TestBuiltinProxy_SidecarMode runs the built-in (non-Envoy) Connect proxy as
// the sidecar of both a client and a server, and verifies that mTLS traffic
// flows and that intentions are enforced by the destination proxy.
func TestBuiltinProxy_SidecarMode(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	ctx := context.Background()

	server, err := libservice.NewExampleService(ctx, libservice.StaticServerServiceName, 8080, 8079, e.clientAgent())
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Terminate() })

	require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:   libservice.StaticServerServiceName,
		Name: libservice.StaticServerServiceName,
		Port: 8080,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{Port: serverSidecarPort},
		},
	}))
	require.NoError(t, e.client.Agent().ServiceRegister(&api.AgentServiceRegistration{
		ID:   libservice.StaticClientServiceName,
		Name: libservice.StaticClientServiceName,
		Port: 8081,
		Connect: &api.AgentServiceConnect{
			SidecarService: &api.AgentServiceRegistration{
				Port: clientSidecarPort,
				Proxy: &api.AgentServiceConnectProxyConfig{
					Upstreams: []api.Upstream{{
						DestinationName:  libservice.StaticServerServiceName,
						LocalBindAddress: "0.0.0.0",
						LocalBindPort:    libcluster.ServiceUpstreamLocalBindPort,
					}},
				},
			},
		},
	}))

	e.startBuiltinProxy(t, "builtin-server-sidecar", libservice.StaticServerServiceName, nil,
		"-sidecar-for", libservice.StaticServerServiceName)
	clientProxy := e.startBuiltinProxy(t, "builtin-client-sidecar", libservice.StaticClientServiceName,
		[]int{libcluster.ServiceUpstreamLocalBindPort},
		"-sidecar-for", libservice.StaticClientServiceName)

	_, port := clientProxy.GetAddr()
	// fortio dumps its environment here, including FORTIO_NAME, which identifies the backend.
	url := fmt.Sprintf("http://localhost:%d/debug?env=dump", port)

	t.Run("traffic flows through both proxies", func(t *testing.T) {
		requireReachable(t, url)

		body, err := httpGet(url)
		require.NoError(t, err)
		require.Contains(t, body, "FORTIO_NAME="+libservice.StaticServerServiceName, "response should come from the upstream service")
	})

	t.Run("sidecar proxies are registered and healthy", func(t *testing.T) {
		retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
			entries, _, err := e.server.Health().Connect(libservice.StaticServerServiceName, "", true, nil)
			require.NoError(r, err)
			require.Len(r, entries, 1)
			require.Equal(r, serverSidecarPort, entries[0].Service.Port)
			require.Equal(r, libservice.StaticServerServiceName, entries[0].Service.Proxy.DestinationServiceName)
		})
	})

	t.Run("a deny intention blocks traffic and removing it restores it", func(t *testing.T) {
		ok, _, err := e.server.ConfigEntries().Set(intentionEntry(libservice.StaticServerServiceName, libservice.StaticClientServiceName, api.IntentionActionDeny), nil)
		require.NoError(t, err)
		require.True(t, ok)
		requireBlocked(t, url)

		_, err = e.server.ConfigEntries().Delete(api.ServiceIntentions, libservice.StaticServerServiceName, nil)
		require.NoError(t, err)
		requireReachable(t, url)
	})

	t.Run("a more specific allow overrides a wildcard deny", func(t *testing.T) {
		ok, _, err := e.server.ConfigEntries().Set(intentionEntry("*", "*", api.IntentionActionDeny), nil)
		require.NoError(t, err)
		require.True(t, ok)
		requireBlocked(t, url)

		ok, _, err = e.server.ConfigEntries().Set(intentionEntry(libservice.StaticServerServiceName, libservice.StaticClientServiceName, api.IntentionActionAllow), nil)
		require.NoError(t, err)
		require.True(t, ok)
		requireReachable(t, url)
	})
}

// TestBuiltinProxy_AdHocMode runs the built-in proxy without a registered
// sidecar: the server side registers itself with -register and the client side
// is described entirely through flags.
func TestBuiltinProxy_AdHocMode(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	ctx := context.Background()

	server, err := libservice.NewExampleService(ctx, "adhoc-backend", 8080, 8079, e.clientAgent())
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Terminate() })

	// -register requires a routable listen address; both proxies share a
	// network namespace, so loopback is reachable from the client side.
	e.startBuiltinProxy(t, "adhoc-server-proxy", "adhoc-backend", nil,
		"-service", "adhoc-backend",
		"-service-addr", "127.0.0.1:8080",
		"-listen", "127.0.0.1:21200",
		"-register", "-register-id", "adhoc-1")

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		entries, _, err := e.server.Health().Connect("adhoc-backend", "", true, nil)
		require.NoError(r, err)
		require.Len(r, entries, 1)
		require.Equal(r, 21200, entries[0].Service.Port)
		require.Equal(r, "127.0.0.1", entries[0].Service.Address)
	})

	clientProxy := e.startBuiltinProxy(t, "adhoc-client-proxy", "adhoc-client", nil,
		"-service", "adhoc-client",
		"-upstream", fmt.Sprintf("adhoc-backend:%d", libcluster.ServiceUpstreamLocalBindPort2))

	// The ad-hoc upstream listens on loopback, so query it from inside the proxy container.
	curl := func() (string, error) {
		return clientProxy.Exec(ctx, []string{"curl", "-sf", "-m", "5", fmt.Sprintf("http://127.0.0.1:%d/debug?env=dump", libcluster.ServiceUpstreamLocalBindPort2)})
	}

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		out, err := curl()
		require.NoError(r, err)
		require.True(r, strings.Contains(out, "FORTIO_NAME=adhoc-backend"), "unexpected response: %s", out)
	})

	ok, _, err := e.server.ConfigEntries().Set(intentionEntry("adhoc-backend", "adhoc-client", api.IntentionActionDeny), nil)
	require.NoError(t, err)
	require.True(t, ok)
	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		_, err := curl()
		require.Error(r, err, "deny intention should block the ad-hoc client")
	})
}
