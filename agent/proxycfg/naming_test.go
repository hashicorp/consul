package proxycfg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestUpstreamIDFromString(t *testing.T) {
	type testcase struct {
		id     string
		expect UpstreamID
	}
	run := func(t *testing.T, tc testcase) {
		tc.expect.EnterpriseMeta.Normalize()

		got := UpstreamIDFromString(tc.id)
		require.Equal(t, tc.expect, got)
	}

	prefix := ""
	if structs.DefaultEnterpriseMetaInDefaultPartition().PartitionOrEmpty() != "" {
		prefix = "default/default/"
	}

	cases := map[string]testcase{
		"prepared query": {
			"prepared_query:" + prefix + "foo",
			UpstreamID{
				Type: structs.UpstreamDestTypePreparedQuery,
				Name: "foo",
			},
		},
		"prepared query dc": {
			"prepared_query:" + prefix + "foo?dc=dc2",
			UpstreamID{
				Type:       structs.UpstreamDestTypePreparedQuery,
				Name:       "foo",
				Datacenter: "dc2",
			},
		},
		"normal": {
			prefix + "foo",
			UpstreamID{
				Name: "foo",
			},
		},
		"normal dc": {
			prefix + "foo?dc=dc2",
			UpstreamID{
				Name:       "foo",
				Datacenter: "dc2",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestUpstreamID_String(t *testing.T) {
	type testcase struct {
		u      UpstreamID
		expect string
	}
	run := func(t *testing.T, tc testcase) {
		got := tc.u.String()
		require.Equal(t, tc.expect, got)
	}

	prefix := ""
	if structs.DefaultEnterpriseMetaInDefaultPartition().PartitionOrEmpty() != "" {
		prefix = "default/default/"
	}

	cases := map[string]testcase{
		"prepared query": {
			UpstreamID{
				Type: structs.UpstreamDestTypePreparedQuery,
				Name: "foo",
			},
			"prepared_query:" + prefix + "foo",
		},
		"prepared query dc": {
			UpstreamID{
				Type:       structs.UpstreamDestTypePreparedQuery,
				Name:       "foo",
				Datacenter: "dc2",
			},
			"prepared_query:" + prefix + "foo?dc=dc2",
		},
		"normal implicit": {
			UpstreamID{
				Name: "foo",
			},
			prefix + "foo",
		},
		"normal implicit dc": {
			UpstreamID{
				Name:       "foo",
				Datacenter: "dc2",
			},
			prefix + "foo?dc=dc2",
		},
		"normal explicit": {
			UpstreamID{
				Type: structs.UpstreamDestTypeService,
				Name: "foo",
			},
			prefix + "foo",
		},
		"normal explicit dc": {
			UpstreamID{
				Type:       structs.UpstreamDestTypeService,
				Name:       "foo",
				Datacenter: "dc2",
			},
			prefix + "foo?dc=dc2",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestUpstreamID_EnvoyID(t *testing.T) {
	type testcase struct {
		u      UpstreamID
		expect string
	}
	run := func(t *testing.T, tc testcase) {
		got := tc.u.EnvoyID()
		require.Equal(t, tc.expect, got)
	}

	cases := map[string]testcase{
		"prepared query": {
			UpstreamID{
				Type: structs.UpstreamDestTypePreparedQuery,
				Name: "foo",
			},
			"prepared_query:foo",
		},
		"prepared query dc": {
			UpstreamID{
				Type:       structs.UpstreamDestTypePreparedQuery,
				Name:       "foo",
				Datacenter: "dc2",
			},
			"prepared_query:foo?dc=dc2",
		},
		"normal implicit": {
			UpstreamID{
				Name: "foo",
			},
			"foo",
		},
		"normal implicit dc": {
			UpstreamID{
				Name:       "foo",
				Datacenter: "dc2",
			},
			"foo?dc=dc2",
		},
		"normal explicit": {
			UpstreamID{
				Type: structs.UpstreamDestTypeService,
				Name: "foo",
			},
			"foo",
		},
		"normal explicit dc": {
			UpstreamID{
				Type:       structs.UpstreamDestTypeService,
				Name:       "foo",
				Datacenter: "dc2",
			},
			"foo?dc=dc2",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
