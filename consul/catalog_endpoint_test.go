package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/rpc"
	"os"
	"sort"
	"testing"
	"time"
)

func TestCatalogRegister(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tag:     "master",
			Port:    8000,
		},
	}
	var out struct{}

	err := client.Call("Catalog.Register", &arg, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogRegister_ForwardLeader(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client1 := rpcClient(t, s1)
	defer client1.Close()

	dir2, s2 := testServer(t)
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()
	client2 := rpcClient(t, s2)
	defer client2.Close()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfLANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinLAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	// Use the follower as the client
	var client *rpc.Client
	if !s1.IsLeader() {
		client = client1
	} else {
		client = client2
	}

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tag:     "master",
			Port:    8000,
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogRegister_ForwardDC(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the leaders
	time.Sleep(100 * time.Millisecond)

	arg := structs.RegisterRequest{
		Datacenter: "dc2", // SHould forward through s1
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			Service: "db",
			Tag:     "master",
			Port:    8000,
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogDeregister(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := structs.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var out struct{}

	err := client.Call("Catalog.Deregister", &arg, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Catalog.Deregister", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestCatalogListDatacenters(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	dir2, s2 := testServerDC(t, "dc2")
	defer os.RemoveAll(dir2)
	defer s2.Shutdown()

	// Try to join
	addr := fmt.Sprintf("127.0.0.1:%d",
		s1.config.SerfWANConfig.MemberlistConfig.BindPort)
	if _, err := s2.JoinWAN([]string{addr}); err != nil {
		t.Fatalf("err: %v", err)
	}
	time.Sleep(10 * time.Millisecond)

	var out []string
	if err := client.Call("Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Sort the dcs
	sort.Strings(out)

	if len(out) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out[0] != "dc1" {
		t.Fatalf("bad: %v", out)
	}
	if out[1] != "dc2" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListNodes(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	var out structs.Nodes
	err := client.Call("Catalog.ListNodes", "dc1", &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(structs.Node{"foo", "127.0.0.1"})

	if err := client.Call("Catalog.ListNodes", "dc1", &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out[0].Node != "foo" {
		t.Fatalf("bad: %v", out)
	}
	if out[0].Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListServices(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	var out structs.Services
	err := client.Call("Catalog.ListServices", "dc1", &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureService("foo", "db", "db", "primary", 5000)

	if err := client.Call("Catalog.ListServices", "dc1", &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if len(out["db"]) != 1 {
		t.Fatalf("bad: %v", out)
	}
	if out["db"][0] != "primary" {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogListServiceNodes(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
		ServiceTag:  "slave",
		TagFilter:   false,
	}
	var out structs.ServiceNodes
	err := client.Call("Catalog.ServiceNodes", &args, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureService("foo", "db", "db", "primary", 5000)

	if err := client.Call("Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(out) != 1 {
		t.Fatalf("bad: %v", out)
	}

	// Try with a filter
	args.TagFilter = true
	out = nil

	if err := client.Call("Catalog.ServiceNodes", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("bad: %v", out)
	}
}

func TestCatalogNodeServices(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	args := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	var out structs.NodeServices
	err := client.Call("Catalog.NodeServices", &args, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	// Just add a node
	s1.fsm.State().EnsureNode(structs.Node{"foo", "127.0.0.1"})
	s1.fsm.State().EnsureService("foo", "db", "db", "primary", 5000)
	s1.fsm.State().EnsureService("foo", "web", "web", "", 80)

	if err := client.Call("Catalog.NodeServices", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	if out.Address != "127.0.0.1" {
		t.Fatalf("bad: %v", out)
	}
	if len(out.Services) != 2 {
		t.Fatalf("bad: %v", out)
	}
	if out.Services["db"].Tag != "primary" || out.Services["db"].Port != 5000 {
		t.Fatalf("bad: %v", out)
	}
	if out.Services["web"].Tag != "" || out.Services["web"].Port != 80 {
		t.Fatalf("bad: %v", out)
	}
}

// Used to check for a regression against a known bug
func TestCatalogRegister_FailedCase1(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "bar",
		Address:    "127.0.0.2",
		Service: &structs.NodeService{
			Service: "web",
			Tag:     "",
			Port:    8000,
		},
	}
	var out struct{}

	err := client.Call("Catalog.Register", &arg, &out)
	if err == nil || err.Error() != "No cluster leader" {
		t.Fatalf("err: %v", err)
	}

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check we can get this back
	query := &structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "web",
	}
	var nodes structs.ServiceNodes
	if err := client.Call("Catalog.ServiceNodes", query, &nodes); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Check the output
	if len(nodes) != 1 {
		t.Fatalf("Bad: %v", nodes)
	}
}
