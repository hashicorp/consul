package connect

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestParseCertURIFromString(t *testing.T) {
	defaultEntMeta := structs.DefaultEnterpriseMetaInDefaultPartition()

	var cases = []struct {
		Name       string
		URI        string
		Struct     interface{}
		ParseError string
	}{
		{
			"invalid scheme",
			"http://google.com/",
			nil,
			"scheme",
		},
		{
			"basic service ID",
			"spiffe://1234.consul/ns/default/dc/dc01/svc/web",
			&SpiffeIDService{
				Host:       "1234.consul",
				Partition:  defaultEntMeta.PartitionOrDefault(),
				Namespace:  "default",
				Datacenter: "dc01",
				Service:    "web",
			},
			"",
		},
		{
			"basic service ID with partition",
			"spiffe://1234.consul/ap/bizdev/ns/default/dc/dc01/svc/web",
			&SpiffeIDService{
				Host:       "1234.consul",
				Partition:  "bizdev",
				Namespace:  "default",
				Datacenter: "dc01",
				Service:    "web",
			},
			"",
		},
		{
			"basic agent ID",
			"spiffe://1234.consul/agent/client/dc/dc1/id/uuid",
			&SpiffeIDAgent{
				Host:       "1234.consul",
				Partition:  defaultEntMeta.PartitionOrDefault(),
				Datacenter: "dc1",
				Agent:      "uuid",
			},
			"",
		},
		{
			"basic agent ID with partition",
			"spiffe://1234.consul/ap/bizdev/agent/client/dc/dc1/id/uuid",
			&SpiffeIDAgent{
				Host:       "1234.consul",
				Partition:  "bizdev",
				Datacenter: "dc1",
				Agent:      "uuid",
			},
			"",
		},
		{
			"mesh-gateway with no partition",
			"spiffe://1234.consul/gateway/mesh/dc/dc1",
			&SpiffeIDMeshGateway{
				Host:       "1234.consul",
				Partition:  "default",
				Datacenter: "dc1",
			},
			"",
		},
		{
			"mesh-gateway with partition",
			"spiffe://1234.consul/ap/bizdev/gateway/mesh/dc/dc1",
			&SpiffeIDMeshGateway{
				Host:       "1234.consul",
				Partition:  "bizdev",
				Datacenter: "dc1",
			},
			"",
		},
		{
			"service with URL-encoded values",
			"spiffe://1234.consul/ns/foo%2Fbar/dc/bar%2Fbaz/svc/baz%2Fqux",
			&SpiffeIDService{
				Host:       "1234.consul",
				Partition:  defaultEntMeta.PartitionOrDefault(),
				Namespace:  "foo/bar",
				Datacenter: "bar/baz",
				Service:    "baz/qux",
			},
			"",
		},
		{
			"service with URL-encoded values with partition",
			"spiffe://1234.consul/ap/biz%2Fdev/ns/foo%2Fbar/dc/bar%2Fbaz/svc/baz%2Fqux",
			&SpiffeIDService{
				Host:       "1234.consul",
				Partition:  "biz/dev",
				Namespace:  "foo/bar",
				Datacenter: "bar/baz",
				Service:    "baz/qux",
			},
			"",
		},
		{
			"signing ID",
			"spiffe://1234.consul",
			&SpiffeIDSigning{
				ClusterID: "1234",
				Domain:    "consul",
			},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual, err := ParseCertURIFromString(tc.URI)
			if tc.ParseError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.ParseError)
				testutil.RequireErrorContains(t, err, tc.ParseError)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.Struct, actual)
			}
		})
	}
}
