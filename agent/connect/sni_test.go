// Copyright (c) HashiCorp, Inc.
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
