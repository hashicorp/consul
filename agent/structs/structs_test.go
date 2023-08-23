package structs

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fuzz "github.com/google/gofuzz"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/types"
)

func TestEncodeDecode(t *testing.T) {
	arg := &RegisterRequest{
		Datacenter: "foo",
		Node:       "bar",
		Address:    "baz",
		Service: &NodeService{
			Service: "test",
			Address: "127.0.0.2",
		},
	}
	buf, err := Encode(RegisterRequestType, arg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	var out RegisterRequest
	err = Decode(buf[1:], &out)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if !reflect.DeepEqual(arg.Service, out.Service) {
		t.Fatalf("bad: %#v %#v", arg.Service, out.Service)
	}
	if !reflect.DeepEqual(arg, &out) {
		t.Fatalf("bad: %#v %#v", arg, out)
	}
}

func TestStructs_Implements(t *testing.T) {
	var (
		_ RPCInfo          = &RegisterRequest{}
		_ RPCInfo          = &DeregisterRequest{}
		_ RPCInfo          = &DCSpecificRequest{}
		_ RPCInfo          = &ServiceSpecificRequest{}
		_ RPCInfo          = &NodeSpecificRequest{}
		_ RPCInfo          = &ChecksInStateRequest{}
		_ RPCInfo          = &KVSRequest{}
		_ RPCInfo          = &KeyRequest{}
		_ RPCInfo          = &KeyListRequest{}
		_ RPCInfo          = &SessionRequest{}
		_ RPCInfo          = &SessionSpecificRequest{}
		_ RPCInfo          = &EventFireRequest{}
		_ RPCInfo          = &ACLPolicyBatchGetRequest{}
		_ RPCInfo          = &ACLPolicyGetRequest{}
		_ RPCInfo          = &ACLTokenGetRequest{}
		_ RPCInfo          = &KeyringRequest{}
		_ CompoundResponse = &KeyringResponses{}
	)
}

func TestStructs_RegisterRequest_ChangesNode(t *testing.T) {

	node := &Node{
		ID:              types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5"),
		Node:            "test",
		Address:         "127.0.0.1",
		Datacenter:      "dc1",
		TaggedAddresses: make(map[string]string),
		Meta: map[string]string{
			"role": "server",
		},
	}

	type testcase struct {
		name   string
		setup  func(*RegisterRequest)
		expect bool
	}

	cases := []testcase{
		{
			name: "id",
			setup: func(r *RegisterRequest) {
				r.ID = "nope"
			},
			expect: true,
		},
		{
			name: "name",
			setup: func(r *RegisterRequest) {
				r.Node = "nope"
			},
			expect: true,
		},
		{
			name: "name casing",
			setup: func(r *RegisterRequest) {
				r.Node = "TeSt"
			},
			expect: false,
		},
		{
			name: "address",
			setup: func(r *RegisterRequest) {
				r.Address = "127.0.0.2"
			},
			expect: true,
		},
		{
			name: "dc",
			setup: func(r *RegisterRequest) {
				r.Datacenter = "dc2"
			},
			expect: true,
		},
		{
			name: "tagged addresses",
			setup: func(r *RegisterRequest) {
				r.TaggedAddresses["wan"] = "nope"
			},
			expect: true,
		},
		{
			name: "node meta",
			setup: func(r *RegisterRequest) {
				r.NodeMeta["invalid"] = "nope"
			},
			expect: true,
		},
	}

	run := func(t *testing.T, tc testcase) {
		req := &RegisterRequest{
			ID:              types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5"),
			Node:            "test",
			Address:         "127.0.0.1",
			Datacenter:      "dc1",
			TaggedAddresses: make(map[string]string),
			NodeMeta: map[string]string{
				"role": "server",
			},
		}

		if req.ChangesNode(node) {
			t.Fatalf("should not change")
		}

		tc.setup(req)

		if tc.expect {
			if !req.ChangesNode(node) {
				t.Fatalf("should change")
			}
		} else {
			if req.ChangesNode(node) {
				t.Fatalf("should not change")
			}
		}

		t.Run("skip node update", func(t *testing.T) {
			req.SkipNodeUpdate = true
			if req.ChangesNode(node) {
				t.Fatalf("should skip")
			}
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

// testServiceNode gives a fully filled out ServiceNode instance.
func testServiceNode(t *testing.T) *ServiceNode {
	return &ServiceNode{
		ID:         types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5"),
		Node:       "node1",
		Address:    "127.0.0.1",
		Datacenter: "dc1",
		TaggedAddresses: map[string]string{
			"hello": "world",
		},
		NodeMeta: map[string]string{
			"tag": "value",
		},
		ServiceKind:    ServiceKindTypical,
		ServiceID:      "service1",
		ServiceName:    "dogs",
		ServiceTags:    []string{"prod", "v1"},
		ServiceAddress: "127.0.0.2",
		ServiceTaggedAddresses: map[string]ServiceAddress{
			"lan": {
				Address: "127.0.0.2",
				Port:    8080,
			},
			"wan": {
				Address: "198.18.0.1",
				Port:    80,
			},
		},
		ServicePort: 8080,
		ServiceMeta: map[string]string{
			"service": "metadata",
		},
		ServiceEnableTagOverride: true,
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
		ServiceProxy: TestConnectProxyConfig(t),
		ServiceConnect: ServiceConnect{
			Native: true,
		},
	}
}

func TestRegisterRequest_UnmarshalJSON_WithConnectNilDoesNotPanic(t *testing.T) {
	in := `
{
    "ID": "",
    "Node": "k8s-sync",
    "Address": "127.0.0.1",
    "TaggedAddresses": null,
    "NodeMeta": {
        "external-source": "kubernetes"
    },
    "Datacenter": "",
    "Service": {
        "Kind": "",
        "ID": "test-service-f8fd5f0f4e6c",
        "Service": "test-service",
        "Tags": [
            "k8s"
        ],
        "Meta": {
            "external-k8s-ns": "",
            "external-source": "kubernetes",
            "port-stats": "18080"
        },
        "Port": 8080,
        "Address": "192.0.2.10",
        "EnableTagOverride": false,
        "CreateIndex": 0,
        "ModifyIndex": 0,
        "Connect": null
    },
    "Check": null,
    "SkipNodeUpdate": true
}
`

	var req RegisterRequest
	err := lib.DecodeJSON(strings.NewReader(in), &req)
	require.NoError(t, err)
}

func TestNode_IsSame(t *testing.T) {
	id := types.NodeID("e62f3b31-9284-4e26-ab14-2a59dea85b55")
	node := "mynode1"
	address := ""
	datacenter := "dc1"
	n := &Node{
		ID:              id,
		Node:            node,
		Datacenter:      datacenter,
		Address:         address,
		TaggedAddresses: make(map[string]string),
		Meta:            make(map[string]string),
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}

	type testcase struct {
		name   string
		setup  func(*Node)
		expect bool
	}
	cases := []testcase{
		{
			name: "id",
			setup: func(n *Node) {
				n.ID = types.NodeID("")
			},
			expect: false,
		},
		{
			name: "node",
			setup: func(n *Node) {
				n.Node = "other"
			},
			expect: false,
		},
		{
			name: "node casing",
			setup: func(n *Node) {
				n.Node = "MyNoDe1"
			},
			expect: true,
		},
		{
			name: "dc",
			setup: func(n *Node) {
				n.Datacenter = "dcX"
			},
			expect: false,
		},
		{
			name: "address",
			setup: func(n *Node) {
				n.Address = "127.0.0.1"
			},
			expect: false,
		},
		{
			name: "tagged addresses",
			setup: func(n *Node) {
				n.TaggedAddresses = map[string]string{"my": "address"}
			},
			expect: false,
		},
		{
			name: "meta",
			setup: func(n *Node) {
				n.Meta = map[string]string{"my": "meta"}
			},
			expect: false,
		},
	}

	run := func(t *testing.T, tc testcase) {
		other := &Node{
			ID:              id,
			Node:            node,
			Datacenter:      datacenter,
			Address:         address,
			TaggedAddresses: make(map[string]string),
			Meta:            make(map[string]string),
			RaftIndex: RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 3,
			},
		}

		if !n.IsSame(other) || !other.IsSame(n) {
			t.Fatalf("should be the same")
		}

		tc.setup(other)

		if tc.expect {
			if !n.IsSame(other) || !other.IsSame(n) {
				t.Fatalf("should be the same")
			}
		} else {
			if n.IsSame(other) || other.IsSame(n) {
				t.Fatalf("should be different, was %#v VS %#v", n, other)
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStructs_ServiceNode_IsSameService(t *testing.T) {
	const (
		nodeName = "node1"
	)

	type testcase struct {
		name   string
		setup  func(*ServiceNode)
		expect bool
	}
	cases := []testcase{
		{
			name: "ServiceID",
			setup: func(sn *ServiceNode) {
				sn.ServiceID = "66fb695a-c782-472f-8d36-4f3edd754b37"
			},
		},
		{
			name: "Node",
			setup: func(sn *ServiceNode) {
				sn.Node = "other"
			},
		},
		{
			name: "Node casing",
			setup: func(sn *ServiceNode) {
				sn.Node = "NoDe1"
			},
			expect: true,
		},
		{
			name: "ServiceAddress",
			setup: func(sn *ServiceNode) {
				sn.ServiceAddress = "1.2.3.4"
			},
		},
		{
			name: "ServiceEnableTagOverride",
			setup: func(sn *ServiceNode) {
				sn.ServiceEnableTagOverride = !sn.ServiceEnableTagOverride
			},
		},
		{
			name: "ServiceKind",
			setup: func(sn *ServiceNode) {
				sn.ServiceKind = "newKind"
			},
		},
		{
			name: "ServiceMeta",
			setup: func(sn *ServiceNode) {
				sn.ServiceMeta = map[string]string{"my": "meta"}
			},
		},
		{
			name: "ServiceName",
			setup: func(sn *ServiceNode) {
				sn.ServiceName = "duck"
			},
		},
		{
			name: "ServicePort",
			setup: func(sn *ServiceNode) {
				sn.ServicePort = 65534
			},
		},
		{
			name: "ServiceTags",
			setup: func(sn *ServiceNode) {
				sn.ServiceTags = []string{"new", "tags"}
			},
		},
		{
			name: "ServiceWeights",
			setup: func(sn *ServiceNode) {
				sn.ServiceWeights = Weights{Passing: 42, Warning: 41}
			},
		},
		{
			name: "ServiceProxy",
			setup: func(sn *ServiceNode) {
				sn.ServiceProxy = ConnectProxyConfig{}
			},
		},
		{
			name: "ServiceConnect",
			setup: func(sn *ServiceNode) {
				sn.ServiceConnect = ServiceConnect{}
			},
		},
		{
			name: "ServiceTaggedAddresses",
			setup: func(sn *ServiceNode) {
				sn.ServiceTaggedAddresses = nil
			},
		},
	}

	run := func(t *testing.T, tc testcase) {
		sn := testServiceNode(t)
		sn.ServiceWeights = Weights{Passing: 2, Warning: 1}
		n := sn.ToNodeService().ToServiceNode(nodeName)
		other := sn.ToNodeService().ToServiceNode(nodeName)

		if !n.IsSameService(other) || !other.IsSameService(n) {
			t.Fatalf("should be the same")
		}

		tc.setup(other)

		if tc.expect {
			if !n.IsSameService(other) || !other.IsSameService(n) {
				t.Fatalf("should be the same")
			}
		} else {
			if n.IsSameService(other) || other.IsSameService(n) {
				t.Fatalf("should be different, was %#v VS %#v", n, other)
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStructs_ServiceNode_PartialClone(t *testing.T) {
	sn := testServiceNode(t)

	clone := sn.PartialClone()

	// Make sure the parts that weren't supposed to be cloned didn't get
	// copied over, then zero-value them out so we can do a DeepEqual() on
	// the rest of the contents.
	if clone.ID != "" ||
		clone.Address != "" ||
		clone.Datacenter != "" ||
		len(clone.TaggedAddresses) != 0 ||
		len(clone.NodeMeta) != 0 {
		t.Fatalf("bad: %v", clone)
	}

	sn.ID = ""
	sn.Address = ""
	sn.Datacenter = ""
	sn.TaggedAddresses = nil
	sn.NodeMeta = nil
	require.Equal(t, sn, clone)

	sn.ServiceTags = append(sn.ServiceTags, "hello")
	if reflect.DeepEqual(sn, clone) {
		t.Fatalf("clone wasn't independent of the original")
	}

	revert := make([]string, len(sn.ServiceTags)-1)
	copy(revert, sn.ServiceTags[0:len(sn.ServiceTags)-1])
	sn.ServiceTags = revert
	if !reflect.DeepEqual(sn, clone) {
		t.Fatalf("bad: %v VS %v", clone, sn)
	}
	oldPassingWeight := clone.ServiceWeights.Passing
	sn.ServiceWeights.Passing = 1000
	if reflect.DeepEqual(sn, clone) {
		t.Fatalf("clone wasn't independent of the original for Meta")
	}
	sn.ServiceWeights.Passing = oldPassingWeight
	sn.ServiceMeta["new_meta"] = "new_value"
	if reflect.DeepEqual(sn, clone) {
		t.Fatalf("clone wasn't independent of the original for Meta")
	}

	// ensure that the tagged addresses were copied and not just a pointer to the map
	sn.ServiceTaggedAddresses["foo"] = ServiceAddress{Address: "consul.is.awesome", Port: 443}
	require.NotEqual(t, sn, clone)
}

func TestStructs_ServiceNode_Conversions(t *testing.T) {
	sn := testServiceNode(t)

	sn2 := sn.ToNodeService().ToServiceNode("node1")

	// These two fields get lost in the conversion, so we have to zero-value
	// them out before we do the compare.
	sn.ID = ""
	sn.Address = ""
	sn.Datacenter = ""
	sn.TaggedAddresses = nil
	sn.NodeMeta = nil
	sn.ServiceWeights = Weights{Passing: 1, Warning: 1}
	require.Equal(t, sn, sn2)
	if !sn.IsSameService(sn2) || !sn2.IsSameService(sn) {
		t.Fatalf("bad: %#v, should be the same %#v", sn2, sn)
	}
	// Those fields are lost in conversion, so IsSameService() should not take them into account
	sn.Address = "y"
	sn.Datacenter = "z"
	sn.TaggedAddresses = map[string]string{"one": "1", "two": "2"}
	sn.NodeMeta = map[string]string{"meta": "data"}
	if !sn.IsSameService(sn2) || !sn2.IsSameService(sn) {
		t.Fatalf("bad: %#v, should be the same %#v", sn2, sn)
	}
}

func TestStructs_NodeService_ValidateMeshGateway(t *testing.T) {
	type testCase struct {
		Modify func(*NodeService)
		Err    string
	}
	cases := map[string]testCase{
		"valid": {
			func(x *NodeService) {},
			"",
		},
		"zero-port": {
			func(x *NodeService) { x.Port = 0 },
			"Port must be non-zero",
		},
		"sidecar-service": {
			func(x *NodeService) { x.Connect.SidecarService = &ServiceDefinition{} },
			"cannot have a sidecar service",
		},
		"proxy-destination-name": {
			func(x *NodeService) { x.Proxy.DestinationServiceName = "foo" },
			"Proxy.DestinationServiceName configuration is invalid",
		},
		"proxy-destination-id": {
			func(x *NodeService) { x.Proxy.DestinationServiceID = "foo" },
			"Proxy.DestinationServiceID configuration is invalid",
		},
		"proxy-local-address": {
			func(x *NodeService) { x.Proxy.LocalServiceAddress = "127.0.0.1" },
			"Proxy.LocalServiceAddress configuration is invalid",
		},
		"proxy-local-port": {
			func(x *NodeService) { x.Proxy.LocalServicePort = 36 },
			"Proxy.LocalServicePort configuration is invalid",
		},
		"proxy-upstreams": {
			func(x *NodeService) { x.Proxy.Upstreams = []Upstream{{}} },
			"Proxy.Upstreams configuration is invalid",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ns := TestNodeServiceMeshGateway(t)
			tc.Modify(ns)

			err := ns.Validate()
			if tc.Err == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
			}
		})
	}
}

func TestStructs_NodeService_ValidateTerminatingGateway(t *testing.T) {
	type testCase struct {
		Modify func(*NodeService)
		Err    string
	}

	cases := map[string]testCase{
		"valid": {
			func(x *NodeService) {},
			"",
		},
		"sidecar-service": {
			func(x *NodeService) { x.Connect.SidecarService = &ServiceDefinition{} },
			"cannot have a sidecar service",
		},
		"proxy-destination-name": {
			func(x *NodeService) { x.Proxy.DestinationServiceName = "foo" },
			"Proxy.DestinationServiceName configuration is invalid",
		},
		"proxy-destination-id": {
			func(x *NodeService) { x.Proxy.DestinationServiceID = "foo" },
			"Proxy.DestinationServiceID configuration is invalid",
		},
		"proxy-local-address": {
			func(x *NodeService) { x.Proxy.LocalServiceAddress = "127.0.0.1" },
			"Proxy.LocalServiceAddress configuration is invalid",
		},
		"proxy-local-port": {
			func(x *NodeService) { x.Proxy.LocalServicePort = 36 },
			"Proxy.LocalServicePort configuration is invalid",
		},
		"proxy-upstreams": {
			func(x *NodeService) { x.Proxy.Upstreams = []Upstream{{}} },
			"Proxy.Upstreams configuration is invalid",
		},
		"port": {
			func(x *NodeService) { x.Port = 0 },
			"Port must be non-zero",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ns := TestNodeServiceTerminatingGateway(t, "10.0.0.5")
			tc.Modify(ns)

			err := ns.Validate()
			if tc.Err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
			}
		})
	}
}

func TestStructs_NodeService_ValidateIngressGateway(t *testing.T) {
	type testCase struct {
		Modify func(*NodeService)
		Err    string
	}

	cases := map[string]testCase{
		"valid": {
			func(x *NodeService) {},
			"",
		},
		"sidecar-service": {
			func(x *NodeService) { x.Connect.SidecarService = &ServiceDefinition{} },
			"cannot have a sidecar service",
		},
		"proxy-destination-name": {
			func(x *NodeService) { x.Proxy.DestinationServiceName = "foo" },
			"Proxy.DestinationServiceName configuration is invalid",
		},
		"proxy-destination-id": {
			func(x *NodeService) { x.Proxy.DestinationServiceID = "foo" },
			"Proxy.DestinationServiceID configuration is invalid",
		},
		"proxy-local-address": {
			func(x *NodeService) { x.Proxy.LocalServiceAddress = "127.0.0.1" },
			"Proxy.LocalServiceAddress configuration is invalid",
		},
		"proxy-local-port": {
			func(x *NodeService) { x.Proxy.LocalServicePort = 36 },
			"Proxy.LocalServicePort configuration is invalid",
		},
		"proxy-upstreams": {
			func(x *NodeService) { x.Proxy.Upstreams = []Upstream{{}} },
			"Proxy.Upstreams configuration is invalid",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ns := TestNodeServiceIngressGateway(t, "10.0.0.5")
			tc.Modify(ns)

			err := ns.Validate()
			if tc.Err == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
			}
		})
	}
}

func TestStructs_NodeService_ValidateExposeConfig(t *testing.T) {
	type testCase struct {
		Modify func(*NodeService)
		Err    string
	}
	cases := map[string]testCase{
		"valid": {
			Modify: func(x *NodeService) {},
			Err:    "",
		},
		"empty path": {
			Modify: func(x *NodeService) { x.Proxy.Expose.Paths[0].Path = "" },
			Err:    "empty path exposed",
		},
		"invalid port negative": {
			Modify: func(x *NodeService) { x.Proxy.Expose.Paths[0].ListenerPort = -1 },
			Err:    "invalid listener port",
		},
		"invalid port too large": {
			Modify: func(x *NodeService) { x.Proxy.Expose.Paths[0].ListenerPort = 65536 },
			Err:    "invalid listener port",
		},
		"duplicate paths are allowed": {
			Modify: func(x *NodeService) {
				x.Proxy.Expose.Paths[0].Path = "/healthz"
				x.Proxy.Expose.Paths[1].Path = "/healthz"
			},
			Err: "",
		},
		"duplicate listener ports are not allowed": {
			Modify: func(x *NodeService) {
				x.Proxy.Expose.Paths[0].ListenerPort = 21600
				x.Proxy.Expose.Paths[1].ListenerPort = 21600
			},
			Err: "duplicate listener ports exposed",
		},
		"protocol not supported": {
			Modify: func(x *NodeService) { x.Proxy.Expose.Paths[0].Protocol = "foo" },
			Err:    "protocol 'foo' not supported for path",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ns := TestNodeServiceExpose(t)
			tc.Modify(ns)

			err := ns.Validate()
			if tc.Err == "" {
				require.NoError(t, err)
			} else {
				require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
			}
		})
	}
}

func TestStructs_NodeService_ValidateConnectProxy(t *testing.T) {
	cases := []struct {
		Name   string
		Modify func(*NodeService)
		Err    string
	}{
		{
			"valid",
			func(x *NodeService) {},
			"",
		},

		{
			"connect-proxy: invalid opaque config",
			func(x *NodeService) {
				x.Proxy.Config = map[string]interface{}{
					"envoy_hcp_metrics_bind_socket_dir": "/Consul/is/a/networking/platform/that/enables/securing/your/networking/",
				}
			},
			"Proxy.Config: envoy_hcp_metrics_bind_socket_dir length 71 exceeds max",
		},

		{
			"connect-proxy: no Proxy.DestinationServiceName",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "" },
			"Proxy.DestinationServiceName must be",
		},

		{
			"connect-proxy: whitespace Proxy.DestinationServiceName",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "  " },
			"Proxy.DestinationServiceName must be",
		},

		{
			"connect-proxy: wildcard Proxy.DestinationServiceName",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "*" },
			"Proxy.DestinationServiceName must not be",
		},

		{
			"connect-proxy: valid Proxy.DestinationServiceName",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "hello" },
			"",
		},

		{
			"connect-proxy: no port set",
			func(x *NodeService) { x.Port = 0 },
			fmt.Sprintf("Port or SocketPath must be set for a %s", ServiceKindConnectProxy),
		},

		{
			"connect-proxy: ConnectNative set",
			func(x *NodeService) { x.Connect.Native = true },
			"cannot also be",
		},

		{
			"connect-proxy: upstream missing type (defaulted)",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationName: "foo",
					LocalBindPort:   5000,
				}}
			},
			"",
		},
		{
			"connect-proxy: upstream invalid type",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationType: "garbage",
					DestinationName: "foo",
					LocalBindPort:   5000,
				}}
			},
			"unknown upstream destination type",
		},
		{
			"connect-proxy: upstream empty name",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationType: UpstreamDestTypeService,
					LocalBindPort:   5000,
				}}
			},
			"upstream destination name cannot be empty",
		},
		{
			"connect-proxy: upstream wildcard name",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationType: UpstreamDestTypeService,
					DestinationName: WildcardSpecifier,
					LocalBindPort:   5000,
				}}
			},
			"upstream destination name cannot be a wildcard",
		},
		{
			"connect-proxy: upstream can have wildcard name when centrally configured",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationType:     UpstreamDestTypeService,
					DestinationName:     WildcardSpecifier,
					CentrallyConfigured: true,
				}}
			},
			"",
		},
		{
			"connect-proxy: upstream empty bind port",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationType: UpstreamDestTypeService,
					DestinationName: "foo",
					LocalBindPort:   0,
				}}
			},
			"upstream local bind port or local socket path must be defined and nonzero",
		},
		{
			"connect-proxy: upstream bind port and path defined",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{{
					DestinationType:     UpstreamDestTypeService,
					DestinationName:     "foo",
					LocalBindPort:       1,
					LocalBindSocketPath: "/tmp/socket",
				}}
			},
			"only one of upstream local bind port or local socket path can be defined and nonzero",
		},
		{
			"connect-proxy: Upstreams almost-but-not-quite-duplicated in various ways",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{ // baseline
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						LocalBindPort:   5000,
					},
					{ // different bind address
						DestinationType:  UpstreamDestTypeService,
						DestinationName:  "bar",
						LocalBindAddress: "127.0.0.2",
						LocalBindPort:    5000,
					},
					{ // different datacenter
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc2",
						LocalBindPort:   5001,
					},
					{ // explicit default namespace
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationNamespace: "default",
						LocalBindPort:        5003,
					},
					{ // different namespace
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationNamespace: "alternate",
						LocalBindPort:        5002,
					},
					{ // different type
						DestinationType: UpstreamDestTypePreparedQuery,
						DestinationName: "foo",
						LocalBindPort:   5004,
					},
				}
			},
			"",
		},
		{
			"connect-proxy: Upstreams non default partition another dc",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{ // baseline
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationPartition: "foo",
						Datacenter:           "dc1",
						LocalBindPort:        5000,
					},
				}
			},
			"upstreams cannot target another datacenter in non default partition",
		},
		{
			"connect-proxy: Upstreams duplicated by port",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						LocalBindPort:   5000,
					},
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						LocalBindPort:   5000,
					},
				}
			},
			"upstreams cannot contain duplicates",
		},
		{
			"connect-proxy: Centrally configured upstreams can have duplicate ip/port",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType:     UpstreamDestTypeService,
						DestinationName:     "foo",
						CentrallyConfigured: true,
					},
					{
						DestinationType:     UpstreamDestTypeService,
						DestinationName:     "bar",
						CentrallyConfigured: true,
					},
				}
			},
			"",
		},
		{
			"connect-proxy: Upstreams duplicated by ip and port",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType:  UpstreamDestTypeService,
						DestinationName:  "foo",
						LocalBindAddress: "127.0.0.2",
						LocalBindPort:    5000,
					},
					{
						DestinationType:  UpstreamDestTypeService,
						DestinationName:  "bar",
						LocalBindAddress: "127.0.0.2",
						LocalBindPort:    5000,
					},
				}
			},
			"upstreams cannot contain duplicates",
		},
		{
			"connect-proxy: Upstreams duplicated by ip and port with ip defaulted in one",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						LocalBindPort:   5000,
					},
					{
						DestinationType:  UpstreamDestTypeService,
						DestinationName:  "foo",
						LocalBindAddress: "127.0.0.1",
						LocalBindPort:    5000,
					},
				}
			},
			"upstreams cannot contain duplicates",
		},
		{
			"connect-proxy: Upstreams duplicated by name",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						LocalBindPort:   5000,
					},
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						LocalBindPort:   5001,
					},
				}
			},
			"upstreams cannot contain duplicates",
		},
		{
			"connect-proxy: Upstreams duplicated by name and datacenter",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc2",
						LocalBindPort:   5000,
					},
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						Datacenter:      "dc2",
						LocalBindPort:   5001,
					},
				}
			},
			"upstreams cannot contain duplicates",
		},
		{
			"connect-proxy: Upstreams duplicated by name and namespace",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationNamespace: "alternate",
						LocalBindPort:        5000,
					},
					{
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationNamespace: "alternate",
						LocalBindPort:        5001,
					},
				}
			},
			"upstreams cannot contain duplicates",
		},
		{
			"connect-proxy: valid Upstream.PeerDestination",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						DestinationPeer: "peer1",
						LocalBindPort:   5000,
					},
				}
			},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ns := TestNodeServiceProxy(t)
			tc.Modify(ns)

			err := ns.Validate()
			assert.Equal(t, err != nil, tc.Err != "", err)
			if err == nil {
				return
			}

			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
		})
	}
}

func TestStructs_NodeService_ValidateConnectProxyWithAgentAutoAssign(t *testing.T) {
	t.Run("connect-proxy: no port set", func(t *testing.T) {
		ns := TestNodeServiceProxy(t)
		ns.Port = 0

		err := ns.ValidateForAgent()
		assert.NoError(t, err)
	})
}

func TestStructs_NodeService_ValidateConnectProxy_In_Partition(t *testing.T) {
	cases := []struct {
		Name   string
		Modify func(*NodeService)
		Err    string
	}{
		{
			"valid",
			func(x *NodeService) {},
			"",
		},
		{
			"connect-proxy: Upstreams non default partition another dc",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{ // baseline
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationPartition: "foo",
						Datacenter:           "dc1",
						LocalBindPort:        5000,
					},
				}
			},
			"upstreams cannot target another datacenter in non default partition",
		},
		{
			"connect-proxy: Upstreams non default partition same dc",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{ // baseline
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationPartition: "foo",
						LocalBindPort:        5000,
					},
				}
			},
			"",
		},
		{
			"connect-proxy: Upstream with peer targets partition different from NodeService",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType:      UpstreamDestTypeService,
						DestinationName:      "foo",
						DestinationPartition: "part1",
						DestinationPeer:      "peer1",
						LocalBindPort:        5000,
					},
				}
			},
			"upstreams must target peers in the same partition as the service",
		},
		{
			"connect-proxy: Upstream with peer defaults to NodeService's peer",
			func(x *NodeService) {
				x.Proxy.Upstreams = Upstreams{
					{
						DestinationType: UpstreamDestTypeService,
						DestinationName: "foo",
						// No DestinationPartition here but we assert that it defaults to "bar" and not "default"
						DestinationPeer: "peer1",
						LocalBindPort:   5000,
					},
				}
			},
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ns := TestNodeServiceProxyInPartition(t, "bar")
			tc.Modify(ns)

			err := ns.Validate()
			assert.Equal(t, err != nil, tc.Err != "", err)
			if err == nil {
				return
			}

			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
		})
	}
}

func TestStructs_NodeService_ValidateSidecarService(t *testing.T) {
	cases := []struct {
		Name   string
		Modify func(*NodeService)
		Err    string
	}{
		{
			"valid",
			func(x *NodeService) {},
			"",
		},

		{
			"ID can't be set",
			func(x *NodeService) { x.Connect.SidecarService.ID = "foo" },
			"SidecarService cannot specify an ID",
		},

		{
			"Nested sidecar can't be set",
			func(x *NodeService) {
				x.Connect.SidecarService.Connect = &ServiceConnect{
					SidecarService: &ServiceDefinition{},
				}
			},
			"SidecarService cannot have a nested SidecarService",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ns := TestNodeServiceSidecar(t)
			tc.Modify(ns)

			err := ns.Validate()
			assert.Equal(t, err != nil, tc.Err != "", err)
			if err == nil {
				return
			}

			assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.Err))
		})
	}
}

func TestStructs_NodeService_ConnectNativeEmptyPortError(t *testing.T) {
	ns := TestNodeService(t)
	ns.Connect.Native = true
	ns.Port = 0
	err := ns.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Port or SocketPath must be set for a Connect native service.")
}

func TestStructs_NodeService_IsSame(t *testing.T) {
	ns := &NodeService{
		ID:      "node1",
		Service: "theservice",
		Tags:    []string{"foo", "bar"},
		Address: "127.0.0.1",
		TaggedAddresses: map[string]ServiceAddress{
			"lan": {
				Address: "127.0.0.1",
				Port:    3456,
			},
			"wan": {
				Address: "198.18.0.1",
				Port:    1234,
			},
		},
		Meta: map[string]string{
			"meta1": "value1",
			"meta2": "value2",
		},
		Port:              1234,
		EnableTagOverride: true,
		Proxy: ConnectProxyConfig{
			DestinationServiceName: "db",
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
		Weights: &Weights{Passing: 1, Warning: 1},
	}
	if !ns.IsSame(ns) {
		t.Fatalf("should be equal to itself")
	}

	other := &NodeService{
		ID:                "node1",
		Service:           "theservice",
		Tags:              []string{"foo", "bar"},
		Address:           "127.0.0.1",
		Port:              1234,
		EnableTagOverride: true,
		TaggedAddresses: map[string]ServiceAddress{
			"wan": {
				Address: "198.18.0.1",
				Port:    1234,
			},
			"lan": {
				Address: "127.0.0.1",
				Port:    3456,
			},
		},
		Meta: map[string]string{
			// We don't care about order
			"meta2": "value2",
			"meta1": "value1",
		},
		Proxy: ConnectProxyConfig{
			DestinationServiceName: "db",
			Config: map[string]interface{}{
				"foo": "bar",
			},
		},
		Weights: &Weights{Passing: 1, Warning: 1},
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}
	if !ns.IsSame(other) || !other.IsSame(ns) {
		t.Fatalf("should not care about Raft fields")
	}

	check := func(twiddle, restore func()) {
		t.Helper()
		if !ns.IsSame(other) || !other.IsSame(ns) {
			t.Fatalf("should be the same")
		}

		twiddle()
		if ns.IsSame(other) || other.IsSame(ns) {
			t.Fatalf("should not be the same")
		}

		restore()
		if !ns.IsSame(other) || !other.IsSame(ns) {
			t.Fatalf("should be the same again")
		}
	}

	check(func() { other.ID = "XXX" }, func() { other.ID = "node1" })
	check(func() { other.Service = "XXX" }, func() { other.Service = "theservice" })
	check(func() { other.Tags = nil }, func() { other.Tags = []string{"foo", "bar"} })
	check(func() { other.Tags = []string{"foo"} }, func() { other.Tags = []string{"foo", "bar"} })
	check(func() { other.Address = "XXX" }, func() { other.Address = "127.0.0.1" })
	check(func() { other.Port = 9999 }, func() { other.Port = 1234 })
	check(func() { other.Meta["meta2"] = "wrongValue" }, func() { other.Meta["meta2"] = "value2" })
	check(func() { other.EnableTagOverride = false }, func() { other.EnableTagOverride = true })
	check(func() { other.Kind = ServiceKindConnectProxy }, func() { other.Kind = "" })
	check(func() { other.Proxy.DestinationServiceName = "" }, func() { other.Proxy.DestinationServiceName = "db" })
	check(func() { other.Proxy.DestinationServiceID = "XXX" }, func() { other.Proxy.DestinationServiceID = "" })
	check(func() { other.Proxy.LocalServiceAddress = "XXX" }, func() { other.Proxy.LocalServiceAddress = "" })
	check(func() { other.Proxy.LocalServicePort = 9999 }, func() { other.Proxy.LocalServicePort = 0 })
	check(func() { other.Proxy.Config["baz"] = "XXX" }, func() { delete(other.Proxy.Config, "baz") })
	check(func() { other.Connect.Native = true }, func() { other.Connect.Native = false })
	otherServiceNode := other.ToServiceNode("node1")
	copyNodeService := otherServiceNode.ToNodeService()
	if !copyNodeService.IsSame(other) {
		t.Fatalf("copy should be the same, but was\n %#v\nVS\n %#v", copyNodeService, other)
	}
	otherServiceNodeCopy2 := copyNodeService.ToServiceNode("node1")
	if !otherServiceNode.IsSameService(otherServiceNodeCopy2) {
		t.Fatalf("copy should be the same, but was\n %#v\nVS\n %#v", otherServiceNode, otherServiceNodeCopy2)
	}
	check(func() { other.TaggedAddresses["lan"] = ServiceAddress{Address: "127.0.0.1", Port: 9999} }, func() { other.TaggedAddresses["lan"] = ServiceAddress{Address: "127.0.0.1", Port: 3456} })
}

func TestStructs_HealthCheck_IsSame(t *testing.T) {
	type testcase struct {
		name   string
		setup  func(*HealthCheck)
		expect bool
	}

	cases := []testcase{
		{
			name: "Node",
			setup: func(hc *HealthCheck) {
				hc.Node = "XXX"
			},
		},
		{
			name: "Node casing",
			setup: func(hc *HealthCheck) {
				hc.Node = "NoDe1"
			},
			expect: true,
		},
		{
			name: "CheckID",
			setup: func(hc *HealthCheck) {
				hc.CheckID = "XXX"
			},
		},
		{
			name: "Name",
			setup: func(hc *HealthCheck) {
				hc.Name = "XXX"
			},
		},
		{
			name: "Status",
			setup: func(hc *HealthCheck) {
				hc.Status = "XXX"
			},
		},
		{
			name: "Notes",
			setup: func(hc *HealthCheck) {
				hc.Notes = "XXX"
			},
		},
		{
			name: "Output",
			setup: func(hc *HealthCheck) {
				hc.Output = "XXX"
			},
		},
		{
			name: "ServiceID",
			setup: func(hc *HealthCheck) {
				hc.ServiceID = "XXX"
			},
		},
		{
			name: "ServiceName",
			setup: func(hc *HealthCheck) {
				hc.ServiceName = "XXX"
			},
		},
	}

	run := func(t *testing.T, tc testcase) {
		hc := &HealthCheck{
			Node:        "node1",
			CheckID:     "check1",
			Name:        "thecheck",
			Status:      api.HealthPassing,
			Notes:       "it's all good",
			Output:      "lgtm",
			ServiceID:   "service1",
			ServiceName: "theservice",
			ServiceTags: []string{"foo"},
		}

		if !hc.IsSame(hc) {
			t.Fatalf("should be equal to itself")
		}

		other := &HealthCheck{
			Node:        "node1",
			CheckID:     "check1",
			Name:        "thecheck",
			Status:      api.HealthPassing,
			Notes:       "it's all good",
			Output:      "lgtm",
			ServiceID:   "service1",
			ServiceName: "theservice",
			ServiceTags: []string{"foo"},
			RaftIndex: RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 2,
			},
		}

		if !hc.IsSame(other) || !other.IsSame(hc) {
			t.Fatalf("should not care about Raft fields")
		}

		tc.setup(hc)

		if tc.expect {
			if !hc.IsSame(other) || !other.IsSame(hc) {
				t.Fatalf("should be the same")
			}
		} else {
			if hc.IsSame(other) || other.IsSame(hc) {
				t.Fatalf("should not be the same")
			}
		}
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}

func TestStructs_HealthCheck_Marshalling(t *testing.T) {
	d := &HealthCheckDefinition{}
	buf, err := d.MarshalJSON()
	require.NoError(t, err)
	require.NotContains(t, string(buf), `"Interval":""`)
	require.NotContains(t, string(buf), `"Timeout":""`)
	require.NotContains(t, string(buf), `"DeregisterCriticalServiceAfter":""`)
}

func TestStructs_HealthCheck_Clone(t *testing.T) {
	hc := &HealthCheck{
		Node:        "node1",
		CheckID:     "check1",
		Name:        "thecheck",
		Status:      api.HealthPassing,
		Notes:       "it's all good",
		Output:      "lgtm",
		ServiceID:   "service1",
		ServiceName: "theservice",
	}
	clone := hc.Clone()
	if !hc.IsSame(clone) {
		t.Fatalf("should be equal to its clone")
	}

	clone.Output = "different"
	if hc.IsSame(clone) {
		t.Fatalf("should not longer be equal to its clone")
	}
}

func TestCheckServiceNodes_Shuffle(t *testing.T) {
	// Make a huge list of nodes.
	var nodes CheckServiceNodes
	for i := 0; i < 100; i++ {
		nodes = append(nodes, CheckServiceNode{
			Node: &Node{
				Node:    fmt.Sprintf("node%d", i),
				Address: fmt.Sprintf("127.0.0.%d", i+1),
			},
		})
	}

	// Keep track of how many unique shuffles we get.
	uniques := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		nodes.Shuffle()

		var names []string
		for _, node := range nodes {
			names = append(names, node.Node.Node)
		}
		key := strings.Join(names, "|")
		uniques[key] = struct{}{}
	}

	// We have to allow for the fact that there won't always be a unique
	// shuffle each pass, so we just look for smell here without the test
	// being flaky.
	if len(uniques) < 50 {
		t.Fatalf("unique shuffle ratio too low: %d/100", len(uniques))
	}
}

func TestCheckServiceNodes_Filter(t *testing.T) {
	nodes := CheckServiceNodes{
		CheckServiceNode{
			Node: &Node{
				Node:    "node1",
				Address: "127.0.0.1",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Status: api.HealthWarning,
				},
			},
		},
		CheckServiceNode{
			Node: &Node{
				Node:    "node2",
				Address: "127.0.0.2",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Status: api.HealthPassing,
				},
			},
		},
		CheckServiceNode{
			Node: &Node{
				Node:    "node3",
				Address: "127.0.0.3",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Status: api.HealthCritical,
				},
			},
		},
		CheckServiceNode{
			Node: &Node{
				Node:    "node4",
				Address: "127.0.0.4",
			},
			Checks: HealthChecks{
				// This check has a different ID to the others to ensure it is not
				// ignored by accident
				&HealthCheck{
					CheckID: "failing2",
					Status:  api.HealthCritical,
				},
			},
		},
	}

	// Test the case where warnings are allowed.
	{
		twiddle := make(CheckServiceNodes, len(nodes))
		if n := copy(twiddle, nodes); n != len(nodes) {
			t.Fatalf("bad: %d", n)
		}
		filtered := twiddle.Filter(false)
		expected := CheckServiceNodes{
			nodes[0],
			nodes[1],
		}
		if !reflect.DeepEqual(filtered, expected) {
			t.Fatalf("bad: %v", filtered)
		}
	}

	// Limit to only passing checks.
	{
		twiddle := make(CheckServiceNodes, len(nodes))
		if n := copy(twiddle, nodes); n != len(nodes) {
			t.Fatalf("bad: %d", n)
		}
		filtered := twiddle.Filter(true)
		expected := CheckServiceNodes{
			nodes[1],
		}
		if !reflect.DeepEqual(filtered, expected) {
			t.Fatalf("bad: %v", filtered)
		}
	}

	// Allow failing checks to be ignored (note that the test checks have empty
	// CheckID which is valid).
	{
		twiddle := make(CheckServiceNodes, len(nodes))
		if n := copy(twiddle, nodes); n != len(nodes) {
			t.Fatalf("bad: %d", n)
		}
		filtered := twiddle.FilterIgnore(true, []types.CheckID{""})
		expected := CheckServiceNodes{
			nodes[0],
			nodes[1],
			nodes[2], // Node 3's critical check should be ignored.
			// Node 4 should still be failing since it's got a critical check with a
			// non-ignored ID.
		}
		if !reflect.DeepEqual(filtered, expected) {
			t.Fatalf("bad: %v", filtered)
		}
	}
}

func TestCheckServiceNode_CanRead(t *testing.T) {
	type testCase struct {
		name     string
		csn      CheckServiceNode
		authz    acl.Authorizer
		expected acl.EnforcementDecision
	}

	fn := func(t *testing.T, tc testCase) {
		actual := tc.csn.CanRead(tc.authz)
		require.Equal(t, tc.expected, actual)
	}

	var testCases = []testCase{
		{
			name:     "empty",
			expected: acl.Deny,
		},
		{
			name: "node read not authorized",
			csn: CheckServiceNode{
				Node:    &Node{Node: "name"},
				Service: &NodeService{Service: "service-name"},
			},
			authz:    aclAuthorizerCheckServiceNode{allowLocalService: true},
			expected: acl.Deny,
		},
		{
			name: "service read not authorized",
			csn: CheckServiceNode{
				Node:    &Node{Node: "name"},
				Service: &NodeService{Service: "service-name"},
			},
			authz:    aclAuthorizerCheckServiceNode{allowLocalNode: true},
			expected: acl.Deny,
		},
		{
			name: "read authorized",
			csn: CheckServiceNode{
				Node:    &Node{Node: "name"},
				Service: &NodeService{Service: "service-name"},
			},
			authz:    acl.AllowAll(),
			expected: acl.Allow,
		},
		{
			name: "can read imported csn if can read imported data",
			csn: CheckServiceNode{
				Node:    &Node{Node: "name", PeerName: "cluster-2"},
				Service: &NodeService{Service: "service-name", PeerName: "cluster-2"},
			},
			authz:    aclAuthorizerCheckServiceNode{allowImported: true},
			expected: acl.Allow,
		},
		{
			name: "can't read imported csn with authz for local services and nodes",
			csn: CheckServiceNode{
				Node:    &Node{Node: "name", PeerName: "cluster-2"},
				Service: &NodeService{Service: "service-name", PeerName: "cluster-2"},
			},
			authz:    aclAuthorizerCheckServiceNode{allowLocalService: true, allowLocalNode: true},
			expected: acl.Deny,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

type aclAuthorizerCheckServiceNode struct {
	acl.Authorizer
	allowLocalNode    bool
	allowLocalService bool
	allowImported     bool
}

func (a aclAuthorizerCheckServiceNode) ServiceRead(_ string, ctx *acl.AuthorizerContext) acl.EnforcementDecision {
	if ctx.Peer != "" {
		if a.allowImported {
			return acl.Allow
		}
		return acl.Deny
	}

	if a.allowLocalService {
		return acl.Allow
	}
	return acl.Deny
}

func (a aclAuthorizerCheckServiceNode) NodeRead(_ string, ctx *acl.AuthorizerContext) acl.EnforcementDecision {
	if ctx.Peer != "" {
		if a.allowImported {
			return acl.Allow
		}
		return acl.Deny
	}

	if a.allowLocalNode {
		return acl.Allow
	}
	return acl.Deny
}

func TestStructs_DirEntry_Clone(t *testing.T) {
	e := &DirEntry{
		LockIndex: 5,
		Key:       "hello",
		Flags:     23,
		Value:     []byte("this is a test"),
		Session:   "session1",
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}

	clone := e.Clone()
	if !reflect.DeepEqual(e, clone) {
		t.Fatalf("bad: %v", clone)
	}

	e.Value = []byte("a new value")
	if reflect.DeepEqual(e, clone) {
		t.Fatalf("clone wasn't independent of the original")
	}
}

func TestStructs_ValidateServiceAndNodeMetadata(t *testing.T) {
	tooMuchMeta := make(map[string]string)
	for i := 0; i < metaMaxKeyPairs+1; i++ {
		tooMuchMeta[fmt.Sprint(i)] = "value"
	}
	type testcase struct {
		Meta              map[string]string
		AllowConsulPrefix bool
		NodeError         string
		ServiceError      string
		GatewayError      string
	}
	cases := map[string]testcase{
		"should succeed": {
			map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			false,
			"",
			"",
			"",
		},
		"invalid key": {
			map[string]string{
				"": "value1",
			},
			false,
			"Couldn't load metadata pair",
			"Couldn't load metadata pair",
			"Couldn't load metadata pair",
		},
		"too many keys": {
			tooMuchMeta,
			false,
			"cannot contain more than",
			"cannot contain more than",
			"cannot contain more than",
		},
		"reserved key prefix denied": {
			map[string]string{
				MetaKeyReservedPrefix + "key": "value1",
			},
			false,
			"reserved for internal use",
			"reserved for internal use",
			"reserved for internal use",
		},
		"reserved key prefix allowed": {
			map[string]string{
				MetaKeyReservedPrefix + "key": "value1",
			},
			true,
			"",
			"",
			"",
		},
		"reserved key prefix allowed via an allowlist just for gateway - " + MetaWANFederationKey: {
			map[string]string{
				MetaWANFederationKey: "value1",
			},
			false,
			"reserved for internal use",
			"reserved for internal use",
			"",
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Run("ValidateNodeMetadata", func(t *testing.T) {
				err := ValidateNodeMetadata(tc.Meta, tc.AllowConsulPrefix)
				if tc.NodeError == "" {
					require.NoError(t, err)
				} else {
					testutil.RequireErrorContains(t, err, tc.NodeError)
				}
			})
			t.Run("ValidateServiceMetadata - typical", func(t *testing.T) {
				err := ValidateServiceMetadata(ServiceKindTypical, tc.Meta, tc.AllowConsulPrefix)
				if tc.ServiceError == "" {
					require.NoError(t, err)
				} else {
					testutil.RequireErrorContains(t, err, tc.ServiceError)
				}
			})
			t.Run("ValidateServiceMetadata - mesh-gateway", func(t *testing.T) {
				err := ValidateServiceMetadata(ServiceKindMeshGateway, tc.Meta, tc.AllowConsulPrefix)
				if tc.GatewayError == "" {
					require.NoError(t, err)
				} else {
					testutil.RequireErrorContains(t, err, tc.GatewayError)
				}
			})
		})
	}
}

func TestStructs_validateMetaPair(t *testing.T) {
	longKey := strings.Repeat("a", metaKeyMaxLength+1)
	longValue := strings.Repeat("b", metaValueMaxLength+1)
	pairs := []struct {
		Key               string
		Value             string
		Error             string
		AllowConsulPrefix bool
		AllowConsulKeys   map[string]struct{}
	}{
		// valid pair
		{"key", "value", "", false, nil},
		// invalid, blank key
		{"", "value", "cannot be blank", false, nil},
		// allowed special chars in key name
		{"k_e-y", "value", "", false, nil},
		// disallowed special chars in key name
		{"(%key&)", "value", "invalid characters", false, nil},
		// key too long
		{longKey, "value", "Key is too long", false, nil},
		// reserved prefix
		{MetaKeyReservedPrefix + "key", "value", "reserved for internal use", false, nil},
		// reserved prefix, allowed
		{MetaKeyReservedPrefix + "key", "value", "", true, nil},
		// reserved prefix, not allowed via an allowlist
		{MetaKeyReservedPrefix + "bad", "value", "reserved for internal use", false, map[string]struct{}{MetaKeyReservedPrefix + "good": {}}},
		// reserved prefix, allowed via an allowlist
		{MetaKeyReservedPrefix + "good", "value", "", true, map[string]struct{}{MetaKeyReservedPrefix + "good": {}}},
		// value too long
		{"key", longValue, "Value is too long", false, nil},
	}

	for _, pair := range pairs {
		err := validateMetaPair(pair.Key, pair.Value, pair.AllowConsulPrefix, pair.AllowConsulKeys)
		if pair.Error == "" && err != nil {
			t.Fatalf("should have succeeded: %v, %v", pair, err)
		} else if pair.Error != "" && !strings.Contains(err.Error(), pair.Error) {
			t.Fatalf("should have failed: %v, %v", pair, err)
		}
	}
}

func TestDCSpecificRequest_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &DCSpecificRequest{})
}

func TestNodeSpecificRequest_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &NodeSpecificRequest{})
}

func TestServiceSpecificRequest_CacheInfoKey(t *testing.T) {
	assertCacheInfoKeyIsComplete(t, &ServiceSpecificRequest{})
}

func TestServiceDumpRequest_CacheInfoKey(t *testing.T) {
	// ServiceKind is only included when UseServiceKind=true
	assertCacheInfoKeyIsComplete(t, &ServiceDumpRequest{}, "ServiceKind")
}

// cacheInfoIgnoredFields are fields that can be ignored in all cache.Request types
// because the cache itself includes these values in the cache key, or because
// they are options used to specify the cache operation, and are not part of the
// cache entry value.
var cacheInfoIgnoredFields = map[string]bool{
	// Datacenter is part of the cache key added by the cache itself.
	"Datacenter": true,
	// PeerName is part of the cache key added by the cache itself.
	"PeerName": true,
	// QuerySource is always the same for every request from a single agent, so it
	// is excluded from the key.
	"Source": true,
	// EnterpriseMeta is an empty struct, so can not be included.
	enterpriseMetaField: true,
}

// assertCacheInfoKeyIsComplete is an assertion to verify that all fields on a request
// struct are considered as part of the cache key. It is used to prevent regressions
// when new fields are added to the struct. If a field is not included in the cache
// key it can lead to API requests or DNS requests returning the wrong value
// because a request matches the wrong entry in the agent/cache.Cache.
func assertCacheInfoKeyIsComplete(t *testing.T, request cache.Request, ignoredFields ...string) {
	t.Helper()

	ignored := make(map[string]bool, len(ignoredFields))
	for _, f := range ignoredFields {
		ignored[f] = true
	}

	fuzzer := fuzz.NewWithSeed(time.Now().UnixNano())
	fuzzer.Funcs(randQueryOptions)
	fuzzer.Fuzz(request)
	requestValue := reflect.ValueOf(request).Elem()

	for i := 0; i < requestValue.NumField(); i++ {
		originalKey := request.CacheInfo().Key
		field := requestValue.Field(i)
		fieldName := requestValue.Type().Field(i).Name
		originalValue := field.Interface()

		if cacheInfoIgnoredFields[fieldName] || ignored[fieldName] {
			continue
		}

		for i := 0; reflect.DeepEqual(originalValue, field.Interface()) && i < 20; i++ {
			fuzzer.Fuzz(field.Addr().Interface())
		}

		key := request.CacheInfo().Key
		if originalKey == key {
			t.Fatalf("expected field %v to be represented in the CacheInfo.Key, %v change to %v (key: %v)",
				fieldName,
				originalValue,
				field.Interface(),
				key)
		}
	}
}

func randQueryOptions(o *QueryOptions, c fuzz.Continue) {
	c.Fuzz(&o.Filter)
}

func TestSpecificServiceRequest_CacheInfo(t *testing.T) {
	tests := []struct {
		name     string
		req      ServiceSpecificRequest
		mutate   func(req *ServiceSpecificRequest)
		want     *cache.RequestInfo
		wantSame bool
	}{
		{
			name: "basic params",
			req: ServiceSpecificRequest{
				QueryOptions: QueryOptions{Token: "foo"},
				Datacenter:   "dc1",
			},
			want: &cache.RequestInfo{
				Token:      "foo",
				Datacenter: "dc1",
			},
			wantSame: true,
		},
		{
			name: "name should be considered",
			req: ServiceSpecificRequest{
				ServiceName: "web",
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.ServiceName = "db"
			},
			wantSame: false,
		},
		{
			name: "node meta should be considered",
			req: ServiceSpecificRequest{
				NodeMetaFilters: map[string]string{
					"foo": "bar",
				},
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.NodeMetaFilters = map[string]string{
					"foo": "qux",
				}
			},
			wantSame: false,
		},
		{
			name: "address should be considered",
			req: ServiceSpecificRequest{
				ServiceAddress: "1.2.3.4",
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.ServiceAddress = "4.3.2.1"
			},
			wantSame: false,
		},
		{
			name: "tag filter should be considered",
			req: ServiceSpecificRequest{
				TagFilter: true,
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.TagFilter = false
			},
			wantSame: false,
		},
		{
			name: "connect should be considered",
			req: ServiceSpecificRequest{
				Connect: true,
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.Connect = false
			},
			wantSame: false,
		},
		{
			name: "tags should be different",
			req: ServiceSpecificRequest{
				ServiceName: "web",
				ServiceTags: []string{"foo"},
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.ServiceTags = []string{"foo", "bar"}
			},
			wantSame: false,
		},
		{
			name: "tags should not depend on order",
			req: ServiceSpecificRequest{
				ServiceName: "web",
				ServiceTags: []string{"bar", "foo"},
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.ServiceTags = []string{"foo", "bar"}
			},
			wantSame: true,
		},
		// DEPRECATED (singular-service-tag) - remove this when upgrade RPC compat
		// with 1.2.x is not required.
		{
			name: "legacy requests with singular tag should be different",
			req: ServiceSpecificRequest{
				ServiceName: "web",
				ServiceTag:  "foo",
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.ServiceTag = "bar"
			},
			wantSame: false,
		},
		{
			name: "with integress=true",
			req: ServiceSpecificRequest{
				Datacenter:  "dc1",
				ServiceName: "my-service",
			},
			mutate: func(req *ServiceSpecificRequest) {
				req.Ingress = true
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := tc.req.CacheInfo()
			if tc.mutate != nil {
				tc.mutate(&tc.req)
			}
			afterInfo := tc.req.CacheInfo()

			// Check key matches or not
			if tc.wantSame {
				require.Equal(t, info, afterInfo)
			} else {
				require.NotEqual(t, info, afterInfo)
			}

			if tc.want != nil {
				// Reset key since we don't care about the actual hash value as long as
				// it does/doesn't change appropriately (asserted with wantSame above).
				info.Key = ""
				require.Equal(t, *tc.want, info)
			}
		})
	}
}

func TestNodeService_JSON_OmitTaggedAdddresses(t *testing.T) {
	cases := []struct {
		name string
		ns   NodeService
	}{
		{
			"nil",
			NodeService{
				TaggedAddresses: nil,
			},
		},
		{
			"empty",
			NodeService{
				TaggedAddresses: make(map[string]ServiceAddress),
			},
		},
	}

	for _, tc := range cases {
		name := tc.name
		ns := tc.ns
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(ns)
			require.NoError(t, err)
			var raw map[string]interface{}
			err = json.Unmarshal(data, &raw)
			require.NoError(t, err)
			require.NotContains(t, raw, "TaggedAddresses")
			require.NotContains(t, raw, "tagged_addresses")
		})
	}
}

func TestServiceNode_JSON_OmitServiceTaggedAdddresses(t *testing.T) {
	cases := []struct {
		name string
		sn   ServiceNode
	}{
		{
			"nil",
			ServiceNode{
				ServiceTaggedAddresses: nil,
			},
		},
		{
			"empty",
			ServiceNode{
				ServiceTaggedAddresses: make(map[string]ServiceAddress),
			},
		},
	}

	for _, tc := range cases {
		name := tc.name
		sn := tc.sn
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(sn)
			require.NoError(t, err)
			var raw map[string]interface{}
			err = json.Unmarshal(data, &raw)
			require.NoError(t, err)
			require.NotContains(t, raw, "ServiceTaggedAddresses")
			require.NotContains(t, raw, "service_tagged_addresses")
		})
	}
}

func TestNode_BestAddress(t *testing.T) {

	type testCase struct {
		input   Node
		lanAddr string
		wanAddr string
	}

	nodeAddr := "10.1.2.3"
	nodeWANAddr := "198.18.19.20"

	cases := map[string]testCase{
		"address": {
			input: Node{
				Address: nodeAddr,
			},

			lanAddr: nodeAddr,
			wanAddr: nodeAddr,
		},
		"wan-address": {
			input: Node{
				Address: nodeAddr,
				TaggedAddresses: map[string]string{
					"wan": nodeWANAddr,
				},
			},

			lanAddr: nodeAddr,
			wanAddr: nodeWANAddr,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {

			require.Equal(t, tc.lanAddr, tc.input.BestAddress(false))
			require.Equal(t, tc.wanAddr, tc.input.BestAddress(true))
		})
	}
}

func TestNodeService_BestAddress(t *testing.T) {

	type testCase struct {
		input   NodeService
		lanAddr string
		lanPort int
		wanAddr string
		wanPort int
	}

	serviceAddr := "10.2.3.4"
	servicePort := 1234
	serviceWANAddr := "198.19.20.21"
	serviceWANPort := 987

	cases := map[string]testCase{
		"no-address": {
			input: NodeService{
				Port: servicePort,
			},

			lanAddr: "",
			lanPort: servicePort,
			wanAddr: "",
			wanPort: servicePort,
		},
		"service-address": {
			input: NodeService{
				Address: serviceAddr,
				Port:    servicePort,
			},

			lanAddr: serviceAddr,
			lanPort: servicePort,
			wanAddr: serviceAddr,
			wanPort: servicePort,
		},
		"service-wan-address": {
			input: NodeService{
				Address: serviceAddr,
				Port:    servicePort,
				TaggedAddresses: map[string]ServiceAddress{
					"wan": {
						Address: serviceWANAddr,
						Port:    serviceWANPort,
					},
				},
			},

			lanAddr: serviceAddr,
			lanPort: servicePort,
			wanAddr: serviceWANAddr,
			wanPort: serviceWANPort,
		},
		"service-wan-address-default-port": {
			input: NodeService{
				Address: serviceAddr,
				Port:    servicePort,
				TaggedAddresses: map[string]ServiceAddress{
					"wan": {
						Address: serviceWANAddr,
						Port:    0,
					},
				},
			},

			lanAddr: serviceAddr,
			lanPort: servicePort,
			wanAddr: serviceWANAddr,
			wanPort: servicePort,
		},
		"service-wan-address-node-lan": {
			input: NodeService{
				Port: servicePort,
				TaggedAddresses: map[string]ServiceAddress{
					"wan": {
						Address: serviceWANAddr,
						Port:    serviceWANPort,
					},
				},
			},

			lanAddr: "",
			lanPort: servicePort,
			wanAddr: serviceWANAddr,
			wanPort: serviceWANPort,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {

			addr, port := tc.input.BestAddress(false)
			require.Equal(t, tc.lanAddr, addr)
			require.Equal(t, tc.lanPort, port)

			addr, port = tc.input.BestAddress(true)
			require.Equal(t, tc.wanAddr, addr)
			require.Equal(t, tc.wanPort, port)
		})
	}
}

func TestCheckServiceNode_BestAddress(t *testing.T) {

	type testCase struct {
		input   CheckServiceNode
		lanAddr string
		lanPort int
		lanIdx  uint64
		wanAddr string
		wanPort int
		wanIdx  uint64
	}

	nodeAddr := "10.1.2.3"
	nodeWANAddr := "198.18.19.20"
	nodeIdx := uint64(11)
	serviceAddr := "10.2.3.4"
	servicePort := 1234
	serviceIdx := uint64(22)
	serviceWANAddr := "198.19.20.21"
	serviceWANPort := 987

	cases := map[string]testCase{
		"node-address": {
			input: CheckServiceNode{
				Node: &Node{
					Address: nodeAddr,
					RaftIndex: RaftIndex{
						ModifyIndex: nodeIdx,
					},
				},
				Service: &NodeService{
					Port: servicePort,
					RaftIndex: RaftIndex{
						ModifyIndex: serviceIdx,
					},
				},
			},

			lanAddr: nodeAddr,
			lanIdx:  nodeIdx,
			lanPort: servicePort,
			wanAddr: nodeAddr,
			wanIdx:  nodeIdx,
			wanPort: servicePort,
		},
		"node-wan-address": {
			input: CheckServiceNode{
				Node: &Node{
					Address: nodeAddr,
					TaggedAddresses: map[string]string{
						"wan": nodeWANAddr,
					},
					RaftIndex: RaftIndex{
						ModifyIndex: nodeIdx,
					},
				},
				Service: &NodeService{
					Port: servicePort,
					RaftIndex: RaftIndex{
						ModifyIndex: serviceIdx,
					},
				},
			},

			lanAddr: nodeAddr,
			lanIdx:  nodeIdx,
			lanPort: servicePort,
			wanAddr: nodeWANAddr,
			wanIdx:  nodeIdx,
			wanPort: servicePort,
		},
		"service-address": {
			input: CheckServiceNode{
				Node: &Node{
					Address: nodeAddr,
					// this will be ignored
					TaggedAddresses: map[string]string{
						"wan": nodeWANAddr,
					},
					RaftIndex: RaftIndex{
						ModifyIndex: nodeIdx,
					},
				},
				Service: &NodeService{
					Address: serviceAddr,
					Port:    servicePort,
					RaftIndex: RaftIndex{
						ModifyIndex: serviceIdx,
					},
				},
			},

			lanAddr: serviceAddr,
			lanIdx:  serviceIdx,
			lanPort: servicePort,
			wanAddr: serviceAddr,
			wanIdx:  serviceIdx,
			wanPort: servicePort,
		},
		"service-wan-address": {
			input: CheckServiceNode{
				Node: &Node{
					Address: nodeAddr,
					// this will be ignored
					TaggedAddresses: map[string]string{
						"wan": nodeWANAddr,
					},
					RaftIndex: RaftIndex{
						ModifyIndex: nodeIdx,
					},
				},
				Service: &NodeService{
					Address: serviceAddr,
					Port:    servicePort,
					TaggedAddresses: map[string]ServiceAddress{
						"wan": {
							Address: serviceWANAddr,
							Port:    serviceWANPort,
						},
					},
					RaftIndex: RaftIndex{
						ModifyIndex: serviceIdx,
					},
				},
			},

			lanAddr: serviceAddr,
			lanIdx:  serviceIdx,
			lanPort: servicePort,
			wanAddr: serviceWANAddr,
			wanIdx:  serviceIdx,
			wanPort: serviceWANPort,
		},
		"service-wan-address-default-port": {
			input: CheckServiceNode{
				Node: &Node{
					Address: nodeAddr,
					// this will be ignored
					TaggedAddresses: map[string]string{
						"wan": nodeWANAddr,
					},
					RaftIndex: RaftIndex{
						ModifyIndex: nodeIdx,
					},
				},
				Service: &NodeService{
					Address: serviceAddr,
					Port:    servicePort,
					TaggedAddresses: map[string]ServiceAddress{
						"wan": {
							Address: serviceWANAddr,
							Port:    0,
						},
					},
					RaftIndex: RaftIndex{
						ModifyIndex: serviceIdx,
					},
				},
			},

			lanAddr: serviceAddr,
			lanIdx:  serviceIdx,
			lanPort: servicePort,
			wanAddr: serviceWANAddr,
			wanIdx:  serviceIdx,
			wanPort: servicePort,
		},
		"service-wan-address-node-lan": {
			input: CheckServiceNode{
				Node: &Node{
					Address: nodeAddr,
					// this will be ignored
					TaggedAddresses: map[string]string{
						"wan": nodeWANAddr,
					},
					RaftIndex: RaftIndex{
						ModifyIndex: nodeIdx,
					},
				},
				Service: &NodeService{
					Port: servicePort,
					TaggedAddresses: map[string]ServiceAddress{
						"wan": {
							Address: serviceWANAddr,
							Port:    serviceWANPort,
						},
					},
					RaftIndex: RaftIndex{
						ModifyIndex: serviceIdx,
					},
				},
			},

			lanAddr: nodeAddr,
			lanIdx:  nodeIdx,
			lanPort: servicePort,
			wanAddr: serviceWANAddr,
			wanIdx:  serviceIdx,
			wanPort: serviceWANPort,
		},
	}

	for name, tc := range cases {
		name := name
		tc := tc
		t.Run(name, func(t *testing.T) {

			idx, addr, port := tc.input.BestAddress(false)
			require.Equal(t, tc.lanAddr, addr)
			require.Equal(t, tc.lanPort, port)
			require.Equal(t, tc.lanIdx, idx)

			idx, addr, port = tc.input.BestAddress(true)
			require.Equal(t, tc.wanAddr, addr)
			require.Equal(t, tc.wanPort, port)
			require.Equal(t, tc.wanIdx, idx)
		})
	}
}

func TestNodeService_JSON_Marshal(t *testing.T) {
	ns := &NodeService{
		Service: "foo",
		Proxy: ConnectProxyConfig{
			Config: map[string]interface{}{
				"bind_addresses": map[string]interface{}{
					"default": map[string]interface{}{
						"Address": "0.0.0.0",
						"Port":    "443",
					},
				},
			},
		},
	}
	buf, err := json.Marshal(ns)
	require.NoError(t, err)

	var out NodeService
	require.NoError(t, json.Unmarshal(buf, &out))
	require.Equal(t, *ns, out)
}

func TestServiceNode_JSON_Marshal(t *testing.T) {
	sn := &ServiceNode{
		Node:        "foo",
		ServiceName: "foo",
		ServiceProxy: ConnectProxyConfig{
			Config: map[string]interface{}{
				"bind_addresses": map[string]interface{}{
					"default": map[string]interface{}{
						"Address": "0.0.0.0",
						"Port":    "443",
					},
				},
			},
		},
	}
	buf, err := json.Marshal(sn)
	require.NoError(t, err)

	var out ServiceNode
	require.NoError(t, json.Unmarshal(buf, &out))
	require.Equal(t, *sn, out)
}

// frankensteinStruct is an amalgamation of all of the different kinds of
// fields you could have on struct defined in the agent/structs package that we
// send through msgpack
type frankensteinStruct struct {
	Child      *monsterStruct
	ChildSlice []*monsterStruct
	ChildMap   map[string]*monsterStruct
}
type monsterStruct struct {
	Bool    bool
	Int     int
	Uint8   uint8
	Uint64  uint64
	Float32 float32
	Float64 float64
	String  string

	Hash         []byte
	Uint32Slice  []uint32
	Float64Slice []float64
	StringSlice  []string

	MapInt         map[string]int
	MapString      map[string]string
	MapStringSlice map[string][]string

	// We explicitly DO NOT try to test the following types that involve
	// interface{} as the TestMsgpackEncodeDecode test WILL fail.
	//
	// These are tested elsewhere for the very specific scenario in question,
	// which usually takes a secondary trip through mapstructure during decode
	// which papers over some of the additional conversions necessary to finish
	// decoding.
	// MapIface    map[string]interface{}
	// MapMapIface map[string]map[string]interface{}

	Dur     time.Duration
	DurPtr  *time.Duration
	Time    time.Time
	TimePtr *time.Time

	RaftIndex
}

func makeFrank() *frankensteinStruct {
	return &frankensteinStruct{
		Child: makeMonster(),
		ChildSlice: []*monsterStruct{
			makeMonster(),
			makeMonster(),
		},
		ChildMap: map[string]*monsterStruct{
			"one": makeMonster(), // only put one key in here so the map order is fixed
		},
	}
}

func makeMonster() *monsterStruct {
	var d time.Duration = 9 * time.Hour
	var t time.Time = time.Date(2008, 1, 2, 3, 4, 5, 0, time.UTC)

	return &monsterStruct{
		Bool:    true,
		Int:     -8,
		Uint8:   5,
		Uint64:  9,
		Float32: 5.25,
		Float64: 99.5,
		String:  "strval",

		Hash:         []byte("hello"),
		Uint32Slice:  []uint32{1, 2, 3, 4},
		Float64Slice: []float64{9.2, 6.25},
		StringSlice:  []string{"foo", "bar"},

		// // MapIface will hold an amalgam of what AuthMethods and
		// // CAConfigurations use in 'Config'
		// MapIface: map[string]interface{}{
		// 	"Name":  "inner",
		// 	"Dur":   "5s",
		// 	"Bool":  true,
		// 	"Float": 15.25,
		// 	"Int":   int64(94),
		// 	"Nested": map[string]string{ // this doesn't survive
		// 		"foo": "bar",
		// 	},
		// },
		// // MapMapIface    map[string]map[string]interface{}

		MapInt: map[string]int{
			"int": 5,
		},
		MapString: map[string]string{
			"aaa": "bbb",
		},
		MapStringSlice: map[string][]string{
			"aaa": {"bbb"},
		},

		Dur:     5 * time.Second,
		DurPtr:  &d,
		Time:    t.Add(-5 * time.Hour),
		TimePtr: &t,

		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 3,
		},
	}
}

func TestStructs_MsgpackEncodeDecode_Monolith(t *testing.T) {
	t.Run("monster", func(t *testing.T) {
		in := makeMonster()
		TestMsgpackEncodeDecode(t, in, false)
	})
	t.Run("frankenstein", func(t *testing.T) {
		in := makeFrank()
		TestMsgpackEncodeDecode(t, in, false)
	})
}

func TestSnapshotRequestResponse_MsgpackEncodeDecode(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		in := &SnapshotRequest{
			Datacenter: "foo",
			Token:      "blah",
			AllowStale: true,
			Op:         SnapshotRestore,
		}
		TestMsgpackEncodeDecode(t, in, true)
	})
	t.Run("response", func(t *testing.T) {
		in := &SnapshotResponse{
			Error: "blah",
			QueryMeta: QueryMeta{
				Index:                 3,
				LastContact:           5 * time.Second,
				KnownLeader:           true,
				ConsistencyLevel:      "default",
				ResultsFilteredByACLs: true,
			},
		}
		TestMsgpackEncodeDecode(t, in, true)
	})

}

func TestGatewayService_IsSame(t *testing.T) {
	gateway := NewServiceName("gateway", nil)
	svc := NewServiceName("web", nil)
	kind := ServiceKindTerminatingGateway
	ca := "ca.pem"
	cert := "client.pem"
	key := "tls.key"
	sni := "mydomain"
	wildcard := false

	g := &GatewayService{
		Gateway:      gateway,
		Service:      svc,
		GatewayKind:  kind,
		CAFile:       ca,
		CertFile:     cert,
		KeyFile:      key,
		SNI:          sni,
		FromWildcard: wildcard,
	}
	other := &GatewayService{
		Gateway:      gateway,
		Service:      svc,
		GatewayKind:  kind,
		CAFile:       ca,
		CertFile:     cert,
		KeyFile:      key,
		SNI:          sni,
		FromWildcard: wildcard,
	}
	check := func(twiddle, restore func()) {
		t.Helper()
		if !g.IsSame(other) || !other.IsSame(g) {
			t.Fatalf("should be the same")
		}

		twiddle()
		if g.IsSame(other) || other.IsSame(g) {
			t.Fatalf("should be different, was %#v VS %#v", g, other)
		}

		restore()
		if !g.IsSame(other) || !other.IsSame(g) {
			t.Fatalf("should be the same")
		}
	}
	check(func() { other.Gateway = NewServiceName("other", nil) }, func() { other.Gateway = gateway })
	check(func() { other.Service = NewServiceName("other", nil) }, func() { other.Service = svc })
	check(func() { other.GatewayKind = ServiceKindIngressGateway }, func() { other.GatewayKind = kind })
	check(func() { other.CAFile = "/certs/cert.pem" }, func() { other.CAFile = ca })
	check(func() { other.CertFile = "/certs/cert.pem" }, func() { other.CertFile = cert })
	check(func() { other.KeyFile = "/certs/cert.pem" }, func() { other.KeyFile = key })
	check(func() { other.SNI = "alt-domain" }, func() { other.SNI = sni })
	check(func() { other.FromWildcard = true }, func() { other.FromWildcard = wildcard })

	if !g.IsSame(other) {
		t.Fatalf("should be equal, was %#v VS %#v", g, other)
	}
}

func TestServiceList_Sort(t *testing.T) {
	type testcase struct {
		name   string
		list   []ServiceName
		expect []ServiceName
	}

	run := func(t *testing.T, tc testcase) {
		t.Run("written order", func(t *testing.T) {
			ServiceList(tc.list).Sort()
			require.Equal(t, tc.expect, tc.list)
		})
		t.Run("random order", func(t *testing.T) {
			rand.Shuffle(len(tc.list), func(i, j int) {
				tc.list[i], tc.list[j] = tc.list[j], tc.list[i]
			})
			ServiceList(tc.list).Sort()
			require.Equal(t, tc.expect, tc.list)
		})
	}

	sn := func(name string) ServiceName {
		return NewServiceName(name, nil)
	}

	cases := []testcase{
		{
			name:   "nil",
			list:   nil,
			expect: nil,
		},
		{
			name:   "empty",
			list:   []ServiceName{},
			expect: []ServiceName{},
		},
		{
			name:   "one",
			list:   []ServiceName{sn("foo")},
			expect: []ServiceName{sn("foo")},
		},
		{
			name: "multiple",
			list: []ServiceName{
				sn("food"),
				sn("zip"),
				sn("Bar"),
				sn("ba"),
				sn("foo"),
				sn("bar"),
				sn("Foo"),
				sn("Zip"),
				sn("foo"),
				sn("bar"),
				sn("barrier"),
			},
			expect: []ServiceName{
				sn("Bar"),
				sn("Foo"),
				sn("Zip"),
				sn("ba"),
				sn("bar"),
				sn("bar"),
				sn("barrier"),
				sn("foo"),
				sn("foo"),
				sn("food"),
				sn("zip"),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
