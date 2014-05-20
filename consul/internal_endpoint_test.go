package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"os"
	"testing"
)

func TestInternal_NodeInfo(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"master"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    structs.HealthPassing,
			ServiceID: "db",
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedNodeDump
	req := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	if err := client.Call("Internal.NodeInfo", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes := out2.Dump
	if len(nodes) != 1 {
		t.Fatalf("Bad: %v", nodes)
	}
	if nodes[0].Node != "foo" {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if !strContains(nodes[0].Services[0].Tags, "master") {
		t.Fatalf("Bad: %v", nodes[0])
	}
	if nodes[0].Checks[0].Status != structs.HealthPassing {
		t.Fatalf("Bad: %v", nodes[0])
	}
}

func TestInternal_NodeDump(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"master"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    structs.HealthPassing,
			ServiceID: "db",
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	arg = structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
			Tags:    []string{"slave"},
		},
		Check: &structs.HealthCheck{
			Name:      "db connect",
			Status:    structs.HealthWarning,
			ServiceID: "db",
		},
	}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.IndexedNodeDump
	req := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	if err := client.Call("Internal.NodeDump", &req, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	nodes := out2.Dump
	if len(nodes) != 3 {
		t.Fatalf("Bad: %v", nodes)
	}

	var foundFoo, foundBar bool
	for _, node := range nodes {
		switch node.Node {
		case "foo":
			foundFoo = true
			if !strContains(node.Services[0].Tags, "master") {
				t.Fatalf("Bad: %v", nodes[0])
			}
			if node.Checks[0].Status != structs.HealthPassing {
				t.Fatalf("Bad: %v", nodes[0])
			}

		case "bar":
			foundBar = true
			if !strContains(node.Services[0].Tags, "slave") {
				t.Fatalf("Bad: %v", nodes[1])
			}
			if node.Checks[0].Status != structs.HealthWarning {
				t.Fatalf("Bad: %v", nodes[1])
			}

		default:
			continue
		}
	}
	if !foundFoo || !foundBar {
		t.Fatalf("missing foo or bar")
	}
}
