package structs

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		_ RPCInfo          = &ACLPolicyResolveLegacyRequest{}
		_ RPCInfo          = &ACLPolicyBatchGetRequest{}
		_ RPCInfo          = &ACLPolicyGetRequest{}
		_ RPCInfo          = &ACLTokenGetRequest{}
		_ RPCInfo          = &KeyringRequest{}
		_ CompoundResponse = &KeyringResponses{}
	)
}

func TestStructs_RegisterRequest_ChangesNode(t *testing.T) {
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

	check := func(twiddle, restore func()) {
		if req.ChangesNode(node) {
			t.Fatalf("should not change")
		}

		twiddle()
		if !req.ChangesNode(node) {
			t.Fatalf("should change")
		}

		req.SkipNodeUpdate = true
		if req.ChangesNode(node) {
			t.Fatalf("should skip")
		}

		req.SkipNodeUpdate = false
		if !req.ChangesNode(node) {
			t.Fatalf("should change")
		}

		restore()
		if req.ChangesNode(node) {
			t.Fatalf("should not change")
		}
	}

	check(func() { req.ID = "nope" }, func() { req.ID = types.NodeID("40e4a748-2192-161a-0510-9bf59fe950b5") })
	check(func() { req.Node = "nope" }, func() { req.Node = "test" })
	check(func() { req.Address = "127.0.0.2" }, func() { req.Address = "127.0.0.1" })
	check(func() { req.Datacenter = "dc2" }, func() { req.Datacenter = "dc1" })
	check(func() { req.TaggedAddresses["wan"] = "nope" }, func() { delete(req.TaggedAddresses, "wan") })
	check(func() { req.NodeMeta["invalid"] = "nope" }, func() { delete(req.NodeMeta, "invalid") })

	if !req.ChangesNode(nil) {
		t.Fatalf("should change")
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
		ServicePort:    8080,
		ServiceMeta: map[string]string{
			"service": "metadata",
		},
		ServiceEnableTagOverride: true,
		RaftIndex: RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
		ServiceProxy: TestConnectProxyConfig(t),
		// DEPRECATED (ProxyDestination) - remove this when removing ProxyDestination
		// ServiceProxyDestination is deprecated bit must be set consistently with
		// the value of ServiceProxy.DestinationServiceName otherwise a round-trip
		// through ServiceNode -> NodeService and back will not match and fail
		// tests.
		ServiceProxyDestination: "web",
		ServiceConnect: ServiceConnect{
			Native: true,
		},
	}
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
	check := func(twiddle, restore func()) {
		t.Helper()
		if !n.IsSame(other) || !other.IsSame(n) {
			t.Fatalf("should be the same")
		}

		twiddle()
		if n.IsSame(other) || other.IsSame(n) {
			t.Fatalf("should be different, was %#v VS %#v", n, other)
		}

		restore()
		if !n.IsSame(other) || !other.IsSame(n) {
			t.Fatalf("should be the same")
		}
	}
	check(func() { other.ID = types.NodeID("") }, func() { other.ID = id })
	check(func() { other.Node = "other" }, func() { other.Node = node })
	check(func() { other.Datacenter = "dcX" }, func() { other.Datacenter = datacenter })
	check(func() { other.Address = "127.0.0.1" }, func() { other.Address = address })
	check(func() { other.TaggedAddresses = map[string]string{"my": "address"} }, func() { other.TaggedAddresses = map[string]string{} })
	check(func() { other.Meta = map[string]string{"my": "meta"} }, func() { other.Meta = map[string]string{} })

	if !n.IsSame(other) {
		t.Fatalf("should be equal, was %#v VS %#v", n, other)
	}
}

func TestStructs_ServiceNode_IsSameService(t *testing.T) {
	sn := testServiceNode(t)
	node := "node1"
	serviceID := sn.ServiceID
	serviceAddress := sn.ServiceAddress
	serviceEnableTagOverride := sn.ServiceEnableTagOverride
	serviceMeta := make(map[string]string)
	for k, v := range sn.ServiceMeta {
		serviceMeta[k] = v
	}
	serviceName := sn.ServiceName
	servicePort := sn.ServicePort
	serviceTags := sn.ServiceTags
	serviceWeights := Weights{Passing: 2, Warning: 1}
	sn.ServiceWeights = serviceWeights
	serviceProxyDestination := sn.ServiceProxyDestination
	serviceProxy := sn.ServiceProxy
	serviceConnect := sn.ServiceConnect

	n := sn.ToNodeService().ToServiceNode(node)
	other := sn.ToNodeService().ToServiceNode(node)

	check := func(twiddle, restore func()) {
		t.Helper()
		if !n.IsSameService(other) || !other.IsSameService(n) {
			t.Fatalf("should be the same")
		}

		twiddle()
		if n.IsSameService(other) || other.IsSameService(n) {
			t.Fatalf("should be different, was %#v VS %#v", n, other)
		}

		restore()
		if !n.IsSameService(other) || !other.IsSameService(n) {
			t.Fatalf("should be the same after restore, was:\n %#v VS\n %#v", n, other)
		}
	}

	check(func() { other.ServiceID = "66fb695a-c782-472f-8d36-4f3edd754b37" }, func() { other.ServiceID = serviceID })
	check(func() { other.Node = "other" }, func() { other.Node = node })
	check(func() { other.ServiceAddress = "1.2.3.4" }, func() { other.ServiceAddress = serviceAddress })
	check(func() { other.ServiceEnableTagOverride = !serviceEnableTagOverride }, func() { other.ServiceEnableTagOverride = serviceEnableTagOverride })
	check(func() { other.ServiceKind = "newKind" }, func() { other.ServiceKind = "" })
	check(func() { other.ServiceMeta = map[string]string{"my": "meta"} }, func() { other.ServiceMeta = serviceMeta })
	check(func() { other.ServiceName = "duck" }, func() { other.ServiceName = serviceName })
	check(func() { other.ServicePort = 65534 }, func() { other.ServicePort = servicePort })
	check(func() { other.ServiceProxyDestination = "duck" }, func() { other.ServiceProxyDestination = serviceProxyDestination })
	check(func() { other.ServiceTags = []string{"new", "tags"} }, func() { other.ServiceTags = serviceTags })
	check(func() { other.ServiceWeights = Weights{Passing: 42, Warning: 41} }, func() { other.ServiceWeights = serviceWeights })
	check(func() { other.ServiceProxy = ConnectProxyConfig{} }, func() { other.ServiceProxy = serviceProxy })
	check(func() { other.ServiceConnect = ServiceConnect{} }, func() { other.ServiceConnect = serviceConnect })
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
			"connect-proxy: no ProxyDestination",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "" },
			"Proxy.DestinationServiceName must be",
		},

		{
			"connect-proxy: whitespace ProxyDestination",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "  " },
			"Proxy.DestinationServiceName must be",
		},

		{
			"connect-proxy: valid ProxyDestination",
			func(x *NodeService) { x.Proxy.DestinationServiceName = "hello" },
			"",
		},

		{
			"connect-proxy: no port set",
			func(x *NodeService) { x.Port = 0 },
			"Port must",
		},

		{
			"connect-proxy: ConnectNative set",
			func(x *NodeService) { x.Connect.Native = true },
			"cannot also be",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			ns := TestNodeServiceProxy(t)
			tc.Modify(ns)

			err := ns.Validate()
			assert.Equal(err != nil, tc.Err != "", err)
			if err == nil {
				return
			}

			assert.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.Err))
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

		{
			"Sidecar can't have managed proxy",
			func(x *NodeService) {
				x.Connect.SidecarService.Connect = &ServiceConnect{
					Proxy: &ServiceDefinitionConnectProxy{},
				}
			},
			"SidecarService cannot have a managed proxy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			assert := assert.New(t)
			ns := TestNodeServiceSidecar(t)
			tc.Modify(ns)

			err := ns.Validate()
			assert.Equal(err != nil, tc.Err != "", err)
			if err == nil {
				return
			}

			assert.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.Err))
		})
	}
}

func TestStructs_NodeService_IsSame(t *testing.T) {
	ns := &NodeService{
		ID:      "node1",
		Service: "theservice",
		Tags:    []string{"foo", "bar"},
		Address: "127.0.0.1",
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
}

func TestStructs_HealthCheck_IsSame(t *testing.T) {
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

	checkCheckIDField := func(field *types.CheckID) {
		if !hc.IsSame(other) || !other.IsSame(hc) {
			t.Fatalf("should be the same")
		}

		old := *field
		*field = "XXX"
		if hc.IsSame(other) || other.IsSame(hc) {
			t.Fatalf("should not be the same")
		}
		*field = old

		if !hc.IsSame(other) || !other.IsSame(hc) {
			t.Fatalf("should be the same")
		}
	}

	checkStringField := func(field *string) {
		if !hc.IsSame(other) || !other.IsSame(hc) {
			t.Fatalf("should be the same")
		}

		old := *field
		*field = "XXX"
		if hc.IsSame(other) || other.IsSame(hc) {
			t.Fatalf("should not be the same")
		}
		*field = old

		if !hc.IsSame(other) || !other.IsSame(hc) {
			t.Fatalf("should be the same")
		}
	}

	checkStringField(&other.Node)
	checkCheckIDField(&other.CheckID)
	checkStringField(&other.Name)
	checkStringField(&other.Status)
	checkStringField(&other.Notes)
	checkStringField(&other.Output)
	checkStringField(&other.ServiceID)
	checkStringField(&other.ServiceName)
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

func TestStructs_CheckServiceNodes_Shuffle(t *testing.T) {
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

func TestStructs_CheckServiceNodes_Filter(t *testing.T) {
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

func TestStructs_ValidateMetadata(t *testing.T) {
	// Load a valid set of key/value pairs
	meta := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	// Should succeed
	if err := ValidateMetadata(meta, false); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Should get error
	meta = map[string]string{
		"": "value1",
	}
	if err := ValidateMetadata(meta, false); !strings.Contains(err.Error(), "Couldn't load metadata pair") {
		t.Fatalf("should have failed")
	}

	// Should get error
	meta = make(map[string]string)
	for i := 0; i < metaMaxKeyPairs+1; i++ {
		meta[string(i)] = "value"
	}
	if err := ValidateMetadata(meta, false); !strings.Contains(err.Error(), "cannot contain more than") {
		t.Fatalf("should have failed")
	}

	// Should not error
	meta = map[string]string{
		metaKeyReservedPrefix + "key": "value1",
	}
	// Should fail
	if err := ValidateMetadata(meta, false); err == nil || !strings.Contains(err.Error(), "reserved for internal use") {
		t.Fatalf("err: %s", err)
	}
	// Should succeed
	if err := ValidateMetadata(meta, true); err != nil {
		t.Fatalf("err: %s", err)
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
	}{
		// valid pair
		{"key", "value", "", false},
		// invalid, blank key
		{"", "value", "cannot be blank", false},
		// allowed special chars in key name
		{"k_e-y", "value", "", false},
		// disallowed special chars in key name
		{"(%key&)", "value", "invalid characters", false},
		// key too long
		{longKey, "value", "Key is too long", false},
		// reserved prefix
		{metaKeyReservedPrefix + "key", "value", "reserved for internal use", false},
		// reserved prefix, allowed
		{metaKeyReservedPrefix + "key", "value", "", true},
		// value too long
		{"key", longValue, "Value is too long", false},
	}

	for _, pair := range pairs {
		err := validateMetaPair(pair.Key, pair.Value, pair.AllowConsulPrefix)
		if pair.Error == "" && err != nil {
			t.Fatalf("should have succeeded: %v, %v", pair, err)
		} else if pair.Error != "" && !strings.Contains(err.Error(), pair.Error) {
			t.Fatalf("should have failed: %v, %v", pair, err)
		}
	}
}
