package consul

import (
	"encoding/base64"
	"fmt"
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

func TestInternal_KeyringOperation(t *testing.T) {
	key1 := "H1dfkSZOVnP/JUnaBfTzXg=="
	keyBytes1, err := base64.StdEncoding.DecodeString(key1)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	dir1, s1 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
	})
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	testutil.WaitForLeader(t, client.Call, "dc1")

	var out structs.KeyringResponses
	req := structs.KeyringRequest{
		Operation:  structs.KeyringList,
		Datacenter: "dc1",
	}
	if err := client.Call("Internal.KeyringOperation", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Two responses (local lan/wan pools) from single-node cluster
	if len(out.Responses) != 2 {
		t.Fatalf("bad: %#v", out)
	}
	if _, ok := out.Responses[0].Keys[key1]; !ok {
		t.Fatalf("bad: %#v", out)
	}
	wanResp, lanResp := 0, 0
	for _, resp := range out.Responses {
		if resp.WAN {
			wanResp++
		} else {
			lanResp++
		}
	}
	if lanResp != 1 || wanResp != 1 {
		t.Fatalf("should have one lan and one wan response")
	}

	// Start a second agent to test cross-dc queries
	dir2, s2 := testServerWithConfig(t, func(c *Config) {
		c.SerfLANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.SerfWANConfig.MemberlistConfig.SecretKey = keyBytes1
		c.Datacenter = "dc2"
	})
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out2 structs.KeyringResponses
	req2 := structs.KeyringRequest{
		Operation: structs.KeyringList,
	}
	if err := client.Call("Internal.KeyringOperation", &req2, &out2); err != nil {
		t.Fatalf("err: %v", err)
	}

	// 3 responses (one from each DC LAN, one from WAN) in two-node cluster
	if len(out2.Responses) != 3 {
		t.Fatalf("bad: %#v", out)
	}
	wanResp, lanResp = 0, 0
	for _, resp := range out2.Responses {
		if resp.WAN {
			wanResp++
		} else {
			lanResp++
		}
	}
	if lanResp != 2 || wanResp != 1 {
		t.Fatalf("should have two lan and one wan response")
	}
}
