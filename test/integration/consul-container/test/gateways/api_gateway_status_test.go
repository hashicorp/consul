// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package gateways

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	libcluster "github.com/hashicorp/consul/test/integration/consul-container/libs/cluster"
)

// entryConditions returns the status conditions the servers' gateway
// controller has published for a gateway or route.
func entryConditions(t retry.TestingTB, c *api.Client, kind, name string) []api.Condition {
	entry, _, err := c.ConfigEntries().Get(kind, name, nil)
	require.NoError(t, err)

	switch e := entry.(type) {
	case *api.APIGatewayConfigEntry:
		return e.Status.Conditions
	case *api.HTTPRouteConfigEntry:
		return e.Status.Conditions
	case *api.TCPRouteConfigEntry:
		return e.Status.Conditions
	}
	require.FailNow(t, "unexpected config entry type", "%T", entry)
	return nil
}

// findCondition returns the condition of the given type that applies to the
// listener section (or to the whole resource when section is empty).
func findCondition(conds []api.Condition, condType, section string) *api.Condition {
	for i, c := range conds {
		var sec string
		if c.Resource != nil {
			sec = c.Resource.SectionName
		}
		if c.Type == condType && sec == section {
			return &conds[i]
		}
	}
	return nil
}

// requireCondition waits for a condition with the given status and reason.
func requireCondition(t *testing.T, c *api.Client, kind, name, condType, section string, status api.ConditionStatus, reason string) {
	t.Helper()

	retry.RunWith(libcluster.LongFailer(), t, func(r *retry.R) {
		cond := findCondition(entryConditions(r, c, kind, name), condType, section)
		require.NotNilf(r, cond, "%s/%s has no %s condition for section %q", kind, name, condType, section)
		require.Equalf(r, status, cond.Status, "%s/%s %s: %s", kind, name, condType, cond.Message)
		require.Equalf(r, reason, cond.Reason, "%s/%s %s: %s", kind, name, condType, cond.Message)
	})
}

func apiGateway(name string, listeners ...api.APIGatewayListener) *api.APIGatewayConfigEntry {
	return &api.APIGatewayConfigEntry{Kind: api.APIGateway, Name: name, Listeners: listeners}
}

func gatewayRef(name, section string) api.ResourceReference {
	return api.ResourceReference{Kind: api.APIGateway, Name: name, SectionName: section}
}

// TestAPIGateway_ConfigEntryValidation verifies the rules servers enforce on
// api-gateway and route entries at write time.
func TestAPIGateway_ConfigEntryValidation(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)

	for name, tc := range map[string]struct {
		entry   api.ConfigEntry
		wantErr string
	}{
		"a gateway without listeners": {
			entry:   apiGateway("no-listeners"),
			wantErr: "must have at least one listener",
		},
		"duplicate listener names": {
			entry: apiGateway("dup-names",
				api.APIGatewayListener{Name: "a", Port: 7001, Protocol: "tcp"},
				api.APIGatewayListener{Name: "a", Port: 7002, Protocol: "tcp"}),
			wantErr: `multiple listeners with the name "a"`,
		},
		"an invalid listener name": {
			entry:   apiGateway("bad-name", api.APIGatewayListener{Name: "not valid!", Port: 7001, Protocol: "tcp"}),
			wantErr: "is invalid, must be at least 1 character and contain only letters, numbers, or dashes",
		},
		"an unsupported protocol": {
			entry:   apiGateway("bad-protocol", api.APIGatewayListener{Name: "a", Port: 7001, Protocol: "udp"}),
			wantErr: `unsupported listener protocol "udp"`,
		},
		"a port out of range": {
			entry:   apiGateway("bad-port", api.APIGatewayListener{Name: "a", Port: 0, Protocol: "tcp"}),
			wantErr: "not in the range 1-65535",
		},
		"a hostname on a tcp listener": {
			entry:   apiGateway("tcp-hostname", api.APIGatewayListener{Name: "a", Port: 7001, Protocol: "tcp", Hostname: "x.example.com"}),
			wantErr: "hostname specification is not supported when using TCP",
		},
		"listeners that cannot share a port": {
			entry: apiGateway("port-clash",
				api.APIGatewayListener{Name: "a", Port: 7001, Protocol: "http"},
				api.APIGatewayListener{Name: "b", Port: 7001, Protocol: "tcp"}),
			wantErr: `listeners "a" and "b" cannot be merged`,
		},
		"an unknown tls version": {
			entry: apiGateway("bad-tls", api.APIGatewayListener{
				Name: "a", Port: 7001, Protocol: "http",
				TLS: api.APIGatewayTLSConfiguration{MinVersion: "TLSv9"},
			}),
			wantErr: "no matching TLS version found for TLSv9",
		},
		"a tcp route with more than one service": {
			entry: &api.TCPRouteConfigEntry{
				Kind:     api.TCPRoute,
				Name:     "two-services",
				Parents:  []api.ResourceReference{gatewayRef("gw", "")},
				Services: []api.TCPService{{Name: "a"}, {Name: "b"}},
			},
			wantErr: "tcp-route currently only supports one service",
		},
	} {
		tc := tc
		t.Run(name, func(t *testing.T) {
			requireSetFails(t, e.server, tc.entry, tc.wantErr)
		})
	}
}

// TestAPIGateway_ListenerAndRouteStatus verifies the status the gateway
// controller publishes as listeners and routes are created, bound, broken and
// repaired. Status is how operators learn why traffic is not flowing, and it
// is computed entirely on the servers, so no data plane is involved.
func TestAPIGateway_ListenerAndRouteStatus(t *testing.T) {
	t.Parallel()

	e := newAPIEnv(t, false)
	c := e.server

	setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "tcp-svc", Protocol: "tcp"})
	setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "http-svc", Protocol: "http"})

	t.Run("a valid gateway is accepted with healthy listeners", func(t *testing.T) {
		setEntry(t, c, apiGateway("status-gw",
			api.APIGatewayListener{Name: "tcp-listener", Port: 7001, Protocol: "tcp"},
			api.APIGatewayListener{Name: "http-listener", Port: 7002, Protocol: "http", Hostname: "*.example.com"},
		))

		requireCondition(t, c, api.APIGateway, "status-gw", "Accepted", "", api.ConditionStatusTrue, "Accepted")
		for _, listener := range []string{"tcp-listener", "http-listener"} {
			requireCondition(t, c, api.APIGateway, "status-gw", "Conflicted", listener, api.ConditionStatusFalse, "NoConflict")
			requireCondition(t, c, api.APIGateway, "status-gw", "ResolvedRefs", listener, api.ConditionStatusTrue, "ResolvedRefs")
		}
	})

	t.Run("tcp and http routes bind to matching listeners", func(t *testing.T) {
		setEntry(t, c, &api.TCPRouteConfigEntry{
			Kind:     api.TCPRoute,
			Name:     "tcp-route",
			Parents:  []api.ResourceReference{gatewayRef("status-gw", "tcp-listener")},
			Services: []api.TCPService{{Name: "tcp-svc"}},
		})
		setEntry(t, c, &api.HTTPRouteConfigEntry{
			Kind:    api.HTTPRoute,
			Name:    "http-route",
			Parents: []api.ResourceReference{gatewayRef("status-gw", "http-listener")},
			Rules:   []api.HTTPRouteRule{{Services: []api.HTTPService{{Name: "http-svc"}}}},
		})

		requireCondition(t, c, api.TCPRoute, "tcp-route", "Accepted", "", api.ConditionStatusTrue, "Accepted")
		requireCondition(t, c, api.TCPRoute, "tcp-route", "Bound", "tcp-listener", api.ConditionStatusTrue, "Bound")
		requireCondition(t, c, api.HTTPRoute, "http-route", "Accepted", "", api.ConditionStatusTrue, "Accepted")
		requireCondition(t, c, api.HTTPRoute, "http-route", "Bound", "http-listener", api.ConditionStatusTrue, "Bound")
	})

	t.Run("a route whose protocol does not match its service is rejected until fixed", func(t *testing.T) {
		setEntry(t, c, apiGateway("mismatch-gw", api.APIGatewayListener{Name: "l", Port: 7003, Protocol: "tcp"}))
		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "flip-svc", Protocol: "http"})
		setEntry(t, c, &api.TCPRouteConfigEntry{
			Kind:     api.TCPRoute,
			Name:     "mismatch-route",
			Parents:  []api.ResourceReference{gatewayRef("mismatch-gw", "")},
			Services: []api.TCPService{{Name: "flip-svc"}},
		})

		requireCondition(t, c, api.TCPRoute, "mismatch-route", "Accepted", "", api.ConditionStatusFalse, "InvalidDiscoveryChain")
		requireCondition(t, c, api.TCPRoute, "mismatch-route", "Bound", "", api.ConditionStatusFalse, "FailedToBind")

		setEntry(t, c, &api.ServiceConfigEntry{Kind: api.ServiceDefaults, Name: "flip-svc", Protocol: "tcp"})

		requireCondition(t, c, api.TCPRoute, "mismatch-route", "Accepted", "", api.ConditionStatusTrue, "Accepted")
		requireCondition(t, c, api.TCPRoute, "mismatch-route", "Bound", "", api.ConditionStatusTrue, "Bound")
	})

	t.Run("a route created before its gateway binds when the gateway appears and unbinds when it goes", func(t *testing.T) {
		setEntry(t, c, &api.TCPRouteConfigEntry{
			Kind:     api.TCPRoute,
			Name:     "early-route",
			Parents:  []api.ResourceReference{gatewayRef("late-gw", "")},
			Services: []api.TCPService{{Name: "tcp-svc"}},
		})
		requireCondition(t, c, api.TCPRoute, "early-route", "Bound", "", api.ConditionStatusFalse, "GatewayNotFound")

		setEntry(t, c, apiGateway("late-gw", api.APIGatewayListener{Name: "l", Port: 7004, Protocol: "tcp"}))
		requireCondition(t, c, api.TCPRoute, "early-route", "Bound", "", api.ConditionStatusTrue, "Bound")

		_, err := c.ConfigEntries().Delete(api.APIGateway, "late-gw", nil)
		require.NoError(t, err)
		requireCondition(t, c, api.TCPRoute, "early-route", "Bound", "", api.ConditionStatusFalse, "GatewayNotFound")
	})

	t.Run("a route can only bind to a listener that exists", func(t *testing.T) {
		setEntry(t, c, &api.HTTPRouteConfigEntry{
			Kind:    api.HTTPRoute,
			Name:    "wrong-section",
			Parents: []api.ResourceReference{gatewayRef("status-gw", "no-such-listener")},
			Rules:   []api.HTTPRouteRule{{Services: []api.HTTPService{{Name: "http-svc"}}}},
		})

		requireCondition(t, c, api.HTTPRoute, "wrong-section", "Bound", "no-such-listener", api.ConditionStatusFalse, "FailedToBind")
	})

	t.Run("a route is bound only when its hostnames fit the listener", func(t *testing.T) {
		setEntry(t, c, &api.HTTPRouteConfigEntry{
			Kind:      api.HTTPRoute,
			Name:      "matching-host",
			Hostnames: []string{"app.example.com"},
			Parents:   []api.ResourceReference{gatewayRef("status-gw", "http-listener")},
			Rules:     []api.HTTPRouteRule{{Services: []api.HTTPService{{Name: "http-svc"}}}},
		})
		setEntry(t, c, &api.HTTPRouteConfigEntry{
			Kind:      api.HTTPRoute,
			Name:      "other-host",
			Hostnames: []string{"app.other.com"},
			Parents:   []api.ResourceReference{gatewayRef("status-gw", "http-listener")},
			Rules:     []api.HTTPRouteRule{{Services: []api.HTTPService{{Name: "http-svc"}}}},
		})

		requireCondition(t, c, api.HTTPRoute, "matching-host", "Bound", "http-listener", api.ConditionStatusTrue, "Bound")
		requireCondition(t, c, api.HTTPRoute, "other-host", "Bound", "http-listener", api.ConditionStatusFalse, "FailedToBind")
	})
}
