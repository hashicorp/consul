// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

const (
	testTrustDomain1 = "5fcd4b81-a2ca-405a-ac62-0fac602c1949.consul"
	testTrustDomain2 = "d2e1a32e-5733-47f2-a9dd-6cf271aab5b7.consul"

	testTrustDomainSuffix1         = internal + ".5fcd4b81-a2ca-405a-ac62-0fac602c1949.consul"
	testTrustDomainSuffix1WithPart = internalVersion + ".5fcd4b81-a2ca-405a-ac62-0fac602c1949.consul"
	testTrustDomainSuffix2         = internal + ".d2e1a32e-5733-47f2-a9dd-6cf271aab5b7.consul"
	testTrustDomainSuffix2WithPart = internalVersion + ".d2e1a32e-5733-47f2-a9dd-6cf271aab5b7.consul"
)

func TestUpstreamSNI(t *testing.T) {
	newup := func(typ, name, ns, dc string) *structs.Upstream {
		u := &structs.Upstream{
			DestinationType:      typ,
			DestinationNamespace: ns,
			DestinationName:      name,
			Datacenter:           dc,
			LocalBindPort:        9999, // required
		}
		require.NoError(t, u.Validate())
		return u
	}

	t.Run("service", func(t *testing.T) {
		// empty namespace, empty subset, empty dc
		require.Equal(t, "api.default.foo."+testTrustDomainSuffix1,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "", "",
			), "", "foo", testTrustDomain1))

		// empty namespace, empty subset, set dc
		require.Equal(t, "api.default.bar."+testTrustDomainSuffix1,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "", "bar",
			), "", "foo", testTrustDomain1))

		// set namespace, empty subset, empty dc
		require.Equal(t, "api.neighbor.foo."+testTrustDomainSuffix2,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "neighbor", "",
			), "", "foo", testTrustDomain2))

		// set namespace, empty subset, set dc
		require.Equal(t, "api.neighbor.bar."+testTrustDomainSuffix2,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "neighbor", "bar",
			), "", "foo", testTrustDomain2))

		// empty namespace, set subset, empty dc
		require.Equal(t, "v2.api.default.foo."+testTrustDomainSuffix1,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "", "",
			), "v2", "foo", testTrustDomain1))

		// empty namespace, set subset, set dc
		require.Equal(t, "v2.api.default.bar."+testTrustDomainSuffix1,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "", "bar",
			), "v2", "foo", testTrustDomain1))

		// set namespace, set subset, empty dc
		require.Equal(t, "canary.api.neighbor.foo."+testTrustDomainSuffix2,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "neighbor", "",
			), "canary", "foo", testTrustDomain2))

		// set namespace, set subset, set dc
		require.Equal(t, "canary.api.neighbor.bar."+testTrustDomainSuffix2,
			UpstreamSNI(newup(structs.UpstreamDestTypeService,
				"api", "neighbor", "bar",
			), "canary", "foo", testTrustDomain2))
	})

	t.Run("prepared query", func(t *testing.T) {
		// empty dc
		require.Equal(t, "magicquery.default.foo.query."+testTrustDomain1,
			UpstreamSNI(newup(structs.UpstreamDestTypePreparedQuery,
				"magicquery", "", "",
			), "", "foo", testTrustDomain1))

		// set dc
		require.Equal(t, "magicquery.default.bar.query."+testTrustDomain2,
			UpstreamSNI(newup(structs.UpstreamDestTypePreparedQuery,
				"magicquery", "", "bar",
			), "", "foo", testTrustDomain2))
	})
}

func TestGatewaySNI(t *testing.T) {
	type testCase struct {
		name        string
		dc          string
		trustDomain string
		expect      string
	}

	run := func(t *testing.T, tc testCase) {
		got := GatewaySNI(tc.dc, "", tc.trustDomain)
		require.Equal(t, tc.expect, got)
	}

	cases := []testCase{
		{
			name:        "foo in domain1",
			dc:          "foo",
			trustDomain: "domain1",
			expect:      "foo.internal.domain1",
		},
		{
			name:        "bar in domain2",
			dc:          "bar",
			trustDomain: "domain2",
			expect:      "bar.internal.domain2",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run(t, c)
		})
	}
}

func TestServiceSNI(t *testing.T) {
	// empty namespace, empty subset
	require.Equal(t, "api.default.foo."+testTrustDomainSuffix1,
		ServiceSNI("api", "", "", "", "foo", testTrustDomain1))

	// set namespace, empty subset
	require.Equal(t, "api.neighbor.foo."+testTrustDomainSuffix2,
		ServiceSNI("api", "", "neighbor", "", "foo", testTrustDomain2))

	// empty namespace, set subset
	require.Equal(t, "v2.api.default.foo."+testTrustDomainSuffix1,
		ServiceSNI("api", "v2", "", "", "foo", testTrustDomain1))

	// set namespace, set subset
	require.Equal(t, "canary.api.neighbor.foo."+testTrustDomainSuffix2,
		ServiceSNI("api", "canary", "neighbor", "", "foo", testTrustDomain2))

	// empty namespace, empty subset, set partition
	require.Equal(t, "api.default.part1.foo."+testTrustDomainSuffix1WithPart,
		ServiceSNI("api", "", "", "part1", "foo", testTrustDomain1))

	// set namespace, empty subset, set partition
	require.Equal(t, "api.neighbor.part1.foo."+testTrustDomainSuffix2WithPart,
		ServiceSNI("api", "", "neighbor", "part1", "foo", testTrustDomain2))

	// empty namespace, set subset, set partition
	require.Equal(t, "v2.api.default.part1.foo."+testTrustDomainSuffix1WithPart,
		ServiceSNI("api", "v2", "", "part1", "foo", testTrustDomain1))

	// set namespace, set subset, set partition
	require.Equal(t, "canary.api.neighbor.part1.foo."+testTrustDomainSuffix2WithPart,
		ServiceSNI("api", "canary", "neighbor", "part1", "foo", testTrustDomain2))
}

func TestPeeredServiceSNI(t *testing.T) {
	require.Equal(t, "api.billing.default.webstuff.external."+testTrustDomainSuffix1,
		PeeredServiceSNI("api", "billing", "", "webstuff", testTrustDomainSuffix1))
}

func TestQuerySNI(t *testing.T) {
	require.Equal(t, "magicquery.default.foo.query."+testTrustDomain1,
		QuerySNI("magicquery", "foo", testTrustDomain1))
}

func TestTargetSNI(t *testing.T) {
	// empty namespace, empty subset
	require.Equal(t, "api.default.foo."+testTrustDomainSuffix1,
		TargetSNI(structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:    "api",
			Partition:  "default",
			Datacenter: "foo",
		}), testTrustDomain1))

	require.Equal(t, "api.default.foo."+testTrustDomainSuffix1,
		TargetSNI(structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:    "api",
			Datacenter: "foo",
		}), testTrustDomain1))

	// set namespace, empty subset
	require.Equal(t, "api.neighbor.foo."+testTrustDomainSuffix2,
		TargetSNI(structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:    "api",
			Namespace:  "neighbor",
			Partition:  "default",
			Datacenter: "foo",
		}), testTrustDomain2))

	// empty namespace, set subset
	require.Equal(t, "v2.api.default.foo."+testTrustDomainSuffix1,
		TargetSNI(structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:       "api",
			ServiceSubset: "v2",
			Partition:     "default",
			Datacenter:    "foo",
		}), testTrustDomain1))

	// set namespace, set subset
	require.Equal(t, "canary.api.neighbor.foo."+testTrustDomainSuffix2,
		TargetSNI(structs.NewDiscoveryTarget(structs.DiscoveryTargetOpts{
			Service:       "api",
			ServiceSubset: "canary",
			Namespace:     "neighbor",
			Partition:     "default",
			Datacenter:    "foo",
		}), testTrustDomain2))
}

func TestClusterNameWithPort(t *testing.T) {
	tests := []struct {
		name     string
		portName string
		sni      string
		expected string
	}{
		{
			name:     "empty port name returns sni unchanged",
			portName: "",
			sni:      "api.default.dc1.internal.consul",
			expected: "api.default.dc1.internal.consul",
		},
		{
			name:     "valid port name prefixes sni",
			portName: "api-port",
			sni:      "api.default.dc1.internal.consul",
			expected: "api-port.api.default.dc1.internal.consul",
		},
		{
			name:     "port name with hyphen",
			portName: "admin-port",
			sni:      "service.default.dc1.internal.consul",
			expected: "admin-port.service.default.dc1.internal.consul",
		},
		{
			name:     "port name with underscore",
			portName: "metrics_port",
			sni:      "service.default.dc1.internal.consul",
			expected: "metrics_port.service.default.dc1.internal.consul",
		},
		{
			name:     "port name with period returns sni unchanged",
			portName: "invalid.port",
			sni:      "api.default.dc1.internal.consul",
			expected: "api.default.dc1.internal.consul",
		},
		{
			name:     "numeric port name",
			portName: "8080",
			sni:      "api.default.dc1.internal.consul",
			expected: "8080.api.default.dc1.internal.consul",
		},
		{
			name:     "alphanumeric port name",
			portName: "port8080",
			sni:      "api.default.dc1.internal.consul",
			expected: "port8080.api.default.dc1.internal.consul",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClusterNameWithPort(tt.portName, tt.sni)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestALPNProtocolForPort(t *testing.T) {
	tests := []struct {
		name     string
		portName string
		expected string
	}{
		{
			name:     "empty port name returns empty string",
			portName: "",
			expected: "",
		},
		{
			name:     "valid port name",
			portName: "api-port",
			expected: "consul~api-port",
		},
		{
			name:     "port name with hyphen",
			portName: "admin-port",
			expected: "consul~admin-port",
		},
		{
			name:     "port name with underscore",
			portName: "metrics_port",
			expected: "consul~metrics_port",
		},
		{
			name:     "numeric port name",
			portName: "8080",
			expected: "consul~8080",
		},
		{
			name:     "alphanumeric port name",
			portName: "port8080",
			expected: "consul~port8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ALPNProtocolForPort(tt.portName)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestALPNRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		portName string
	}{
		{
			name:     "api-port",
			portName: "api-port",
		},
		{
			name:     "admin-port",
			portName: "admin-port",
		},
		{
			name:     "metrics_port",
			portName: "metrics_port",
		},
		{
			name:     "numeric port",
			portName: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alpn := ALPNProtocolForPort(tt.portName)
			require.Equal(t, "consul~"+tt.portName, alpn)
		})
	}
}

func TestClusterNameWithPortIntegration(t *testing.T) {
	// Test integration with ServiceSNI
	sni := ServiceSNI("api", "", "default", "", "dc1", testTrustDomain1)
	require.Equal(t, "api.default.dc1."+testTrustDomainSuffix1, sni)

	// Add port prefix
	clusterName := ClusterNameWithPort("api-port", sni)
	require.Equal(t, "api-port.api.default.dc1."+testTrustDomainSuffix1, clusterName)

	// Verify backward compatibility (empty port name)
	clusterNameCompat := ClusterNameWithPort("", sni)
	require.Equal(t, sni, clusterNameCompat)
}
