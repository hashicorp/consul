// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

// TestSynthesizeHTTPRouteDiscoveryChain_UpstreamRoutingComposition validates the
// #12706 behavior at the synthesizeHTTPRouteDiscoveryChain level — the
// function that builds the intermediate ServiceRouterConfigEntry before full
// chain compilation.
//
// This is the correct level to test because:
//   - disabled (legacy): routes have no HTTP match and no service subset.
//   - enabled  (#12706): routes carry the HTTPRoute path match merged with
//     the service-router rule, and the canary subset is preserved.
//
// Full chain compilation is not exercised here because it requires a resolver
// config entry for every subset, which is out of scope for a gate-behavior test.
func TestSynthesizeHTTPRouteDiscoveryChain_UpstreamRoutingComposition(t *testing.T) {
	t.Parallel()

	route := structs.HTTPRouteConfigEntry{
		Kind: structs.HTTPRoute,
		Name: "route",
		Rules: []structs.HTTPRouteRule{{
			Matches: []structs.HTTPMatch{{
				Path: structs.HTTPPathMatch{
					Match: structs.HTTPPathMatchPrefix,
					Value: "/api",
				},
			}},
			Services: []structs.HTTPService{{Name: "backend"}},
		}},
	}

	// serviceRouters carries a canary subset rule for "backend", as would
	// be extracted from a compiled discovery chain by #12706's
	// serviceRouterRulesFromChains.
	serviceRouters := map[structs.ServiceName][]*structs.ServiceRoute{
		structs.NewServiceName("backend", nil): {{
			Destination: &structs.ServiceRouteDestination{ServiceSubset: "canary"},
		}},
	}

	t.Run("disabled: HTTPRoute match preserved, no service subset (legacy shape)", func(t *testing.T) {
		_, router, _, _ := synthesizeHTTPRouteDiscoveryChain(route, serviceRouters, false)
		require.Len(t, router.Routes, 1)
		// Legacy: the HTTPRoute match is passed through directly — no service-router
		// rules are composed.  The route is NOT a catch-all; it mirrors the
		// HTTPRoute path match exactly as the pre-#12706 code did.
		require.NotNil(t, router.Routes[0].Match,
			"disabled: HTTPRoute match must be passed through")
		require.Equal(t, "/api", router.Routes[0].Match.HTTP.PathPrefix,
			"disabled: path prefix must mirror the HTTPRoute match value")
		// Legacy: the canary subset from the service-router is not composed in.
		require.Empty(t, router.Routes[0].Destination.ServiceSubset,
			"disabled: canary subset must not appear in destination")
	})

	t.Run("enabled: path match + canary subset composed (#12706 shape)", func(t *testing.T) {
		_, router, _, _ := synthesizeHTTPRouteDiscoveryChain(route, serviceRouters, true)
		require.Len(t, router.Routes, 1)
		// #12706: route carries the HTTPRoute path match.
		require.NotNil(t, router.Routes[0].Match,
			"enabled: route must carry an HTTP match")
		require.Equal(t, "/api", router.Routes[0].Match.HTTP.PathPrefix,
			"enabled: path prefix must equal the HTTPRoute match value")
		// #12706: service-router canary subset is merged into the destination.
		require.Equal(t, "canary", router.Routes[0].Destination.ServiceSubset,
			"enabled: canary subset from service-router must be preserved")
	})

	t.Run("enabled: no-match rule gets default prefix-/ match applied", func(t *testing.T) {
		noMatchRoute := structs.HTTPRouteConfigEntry{
			Kind: structs.HTTPRoute,
			Name: "no-match-route",
			Rules: []structs.HTTPRouteRule{{
				// No Matches field — should get the default "/" prefix.
				Services: []structs.HTTPService{{Name: "backend"}},
			}},
		}
		_, router, _, _ := synthesizeHTTPRouteDiscoveryChain(noMatchRoute, serviceRouters, true)
		require.Len(t, router.Routes, 1)
		require.Equal(t, "/", router.Routes[0].Match.HTTP.PathPrefix,
			"enabled: rule with no match must default to PathPrefix=/")
		require.Equal(t, "canary", router.Routes[0].Destination.ServiceSubset)
	})

	t.Run("enabled: upstream with no service-router rules passes through unchanged", func(t *testing.T) {
		_, router, _, _ := synthesizeHTTPRouteDiscoveryChain(route, nil, true)
		require.Len(t, router.Routes, 1)
		require.NotNil(t, router.Routes[0].Match,
			"enabled: match still applied even with no service-router rules")
		require.Empty(t, router.Routes[0].Destination.ServiceSubset,
			"no service-router rules: no subset to compose")
	})
}

func TestHTTPRouteToDiscoveryChain_UpstreamRoutingCompositionGate(t *testing.T) {
	route := structs.HTTPRouteConfigEntry{
		Kind: structs.HTTPRoute,
		Name: "route",
		Rules: []structs.HTTPRouteRule{{
			Services: []structs.HTTPService{{Name: "backend"}},
		}},
	}
	routers := map[structs.ServiceName][]*structs.ServiceRoute{
		structs.NewServiceName("backend", nil): {{
			Destination: &structs.ServiceRouteDestination{ServiceSubset: "canary"},
		}},
	}

	legacy, _, _ := httpRouteToDiscoveryChain(route, routers, false)
	require.Len(t, legacy.Routes, 1)
	require.Nil(t, legacy.Routes[0].Match)
	require.Empty(t, legacy.Routes[0].Destination.ServiceSubset)

	composed, _, _ := httpRouteToDiscoveryChain(route, routers, true)
	require.Len(t, composed.Routes, 1)
	require.Equal(t, "/", composed.Routes[0].Match.HTTP.PathPrefix)
	require.Equal(t, "canary", composed.Routes[0].Destination.ServiceSubset)
}
