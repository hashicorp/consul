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
			Name:       "invalid scheme",
			URI:        "http://google.com/",
			Struct:     nil,
			ParseError: "scheme",
		},
		{
			Name: "basic service ID",
			URI:  "spiffe://1234.consul/ns/default/dc/dc01/svc/web",
			Struct: &SpiffeIDService{
				Host:       "1234.consul",
				Partition:  defaultEntMeta.PartitionOrDefault(),
				Namespace:  "default",
				Datacenter: "dc01",
				Service:    "web",
			},
			ParseError: "",
		},
		{
			Name: "basic service ID with partition",
			URI:  "spiffe://1234.consul/ap/bizdev/ns/default/dc/dc01/svc/web",
			Struct: &SpiffeIDService{
				Host:       "1234.consul",
				Partition:  "bizdev",
				Namespace:  "default",
				Datacenter: "dc01",
				Service:    "web",
			},
			ParseError: "",
		},
		{
			Name: "basic agent ID",
			URI:  "spiffe://1234.consul/agent/client/dc/dc1/id/uuid",
			Struct: &SpiffeIDAgent{
				Host:       "1234.consul",
				Partition:  defaultEntMeta.PartitionOrDefault(),
				Datacenter: "dc1",
				Agent:      "uuid",
			},
			ParseError: "",
		},
		{
			Name: "basic agent ID with partition",
			URI:  "spiffe://1234.consul/ap/bizdev/agent/client/dc/dc1/id/uuid",
			Struct: &SpiffeIDAgent{
				Host:       "1234.consul",
				Partition:  "bizdev",
				Datacenter: "dc1",
				Agent:      "uuid",
			},
			ParseError: "",
		},
		{
			Name: "basic server",
			URI:  "spiffe://1234.consul/agent/server/dc/dc1",
			Struct: &SpiffeIDServer{
				Host:       "1234.consul",
				Datacenter: "dc1",
			},
			ParseError: "",
		},
		{
			Name: "mesh-gateway with no partition",
			URI:  "spiffe://1234.consul/gateway/mesh/dc/dc1",
			Struct: &SpiffeIDMeshGateway{
				Host:       "1234.consul",
				Partition:  "default",
				Datacenter: "dc1",
			},
			ParseError: "",
		},
		{
			Name: "mesh-gateway with partition",
			URI:  "spiffe://1234.consul/ap/bizdev/gateway/mesh/dc/dc1",
			Struct: &SpiffeIDMeshGateway{
				Host:       "1234.consul",
				Partition:  "bizdev",
				Datacenter: "dc1",
			},
			ParseError: "",
		},
		{
			Name: "service with URL-encoded values",
			URI:  "spiffe://1234.consul/ns/foo%2Fbar/dc/bar%2Fbaz/svc/baz%2Fqux",
			Struct: &SpiffeIDService{
				Host:       "1234.consul",
				Partition:  defaultEntMeta.PartitionOrDefault(),
				Namespace:  "foo/bar",
				Datacenter: "bar/baz",
				Service:    "baz/qux",
			},
			ParseError: "",
		},
		{
			Name: "service with URL-encoded values with partition",
			URI:  "spiffe://1234.consul/ap/biz%2Fdev/ns/foo%2Fbar/dc/bar%2Fbaz/svc/baz%2Fqux",
			Struct: &SpiffeIDService{
				Host:       "1234.consul",
				Partition:  "biz/dev",
				Namespace:  "foo/bar",
				Datacenter: "bar/baz",
				Service:    "baz/qux",
			},
			ParseError: "",
		},
		{
			Name: "signing ID",
			URI:  "spiffe://1234.consul",
			Struct: &SpiffeIDSigning{
				ClusterID: "1234",
				Domain:    "consul",
			},
			ParseError: "",
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

func TestSpiffeIDServer_URI(t *testing.T) {
	srv := &SpiffeIDServer{
		Host:       "1234.consul",
		Datacenter: "dc1",
	}

	require.Equal(t, "spiffe://1234.consul/agent/server/dc/dc1", srv.URI().String())
}

func TestServerSAN(t *testing.T) {
	san := PeeringServerSAN("dc1", TestTrustDomain)
	expect := "server.dc1.peering." + TestTrustDomain
	require.Equal(t, expect, san)
}
