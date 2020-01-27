package connect

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

const (
	testTrustDomain1 = "5fcd4b81-a2ca-405a-ac62-0fac602c1949.consul"
	testTrustDomain2 = "d2e1a32e-5733-47f2-a9dd-6cf271aab5b7.consul"

	testTrustDomainSuffix1 = "internal.5fcd4b81-a2ca-405a-ac62-0fac602c1949.consul"
	testTrustDomainSuffix2 = "internal.d2e1a32e-5733-47f2-a9dd-6cf271aab5b7.consul"
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

func TestDatacenterSNI(t *testing.T) {
	require.Equal(t, "foo."+testTrustDomainSuffix1, DatacenterSNI("foo", testTrustDomain1))
	require.Equal(t, "bar."+testTrustDomainSuffix2, DatacenterSNI("bar", testTrustDomain2))
}

func TestServiceSNI(t *testing.T) {
	// empty namespace, empty subset
	require.Equal(t, "api.default.foo."+testTrustDomainSuffix1,
		ServiceSNI("api", "", "", "foo", testTrustDomain1))

	// set namespace, empty subset
	require.Equal(t, "api.neighbor.foo."+testTrustDomainSuffix2,
		ServiceSNI("api", "", "neighbor", "foo", testTrustDomain2))

	// empty namespace, set subset
	require.Equal(t, "v2.api.default.foo."+testTrustDomainSuffix1,
		ServiceSNI("api", "v2", "", "foo", testTrustDomain1))

	// set namespace, set subset
	require.Equal(t, "canary.api.neighbor.foo."+testTrustDomainSuffix2,
		ServiceSNI("api", "canary", "neighbor", "foo", testTrustDomain2))
}

func TestQuerySNI(t *testing.T) {
	require.Equal(t, "magicquery.default.foo.query."+testTrustDomain1,
		QuerySNI("magicquery", "foo", testTrustDomain1))
}

func TestTargetSNI(t *testing.T) {
	// empty namespace, empty subset
	require.Equal(t, "api.default.foo."+testTrustDomainSuffix1,
		TargetSNI(structs.NewDiscoveryTarget("api", "", "", "foo"), testTrustDomain1))

	// set namespace, empty subset
	require.Equal(t, "api.neighbor.foo."+testTrustDomainSuffix2,
		TargetSNI(structs.NewDiscoveryTarget("api", "", "neighbor", "foo"), testTrustDomain2))

	// empty namespace, set subset
	require.Equal(t, "v2.api.default.foo."+testTrustDomainSuffix1,
		TargetSNI(structs.NewDiscoveryTarget("api", "v2", "", "foo"), testTrustDomain1))

	// set namespace, set subset
	require.Equal(t, "canary.api.neighbor.foo."+testTrustDomainSuffix2,
		TargetSNI(structs.NewDiscoveryTarget("api", "canary", "neighbor", "foo"), testTrustDomain2))
}
