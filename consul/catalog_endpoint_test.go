package consul

import (
	"fmt"
	"github.com/hashicorp/consul/rpc"
	nrpc "net/rpc"
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

	arg := rpc.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
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
		s1.config.SerfLANConfig.MemberlistConfig.Port)
	if err := s2.JoinLAN(addr); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for a leader
	time.Sleep(100 * time.Millisecond)

	// Use the follower as the client
	var client *nrpc.Client
	if !s1.IsLeader() {
		client = client1
	} else {
		client = client2
	}

	arg := rpc.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
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
		s1.config.SerfWANConfig.MemberlistConfig.Port)
	if err := s2.JoinWAN(addr); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Wait for the leaders
	time.Sleep(100 * time.Millisecond)

	arg := rpc.RegisterRequest{
		Datacenter:  "dc2", // SHould forward through s1
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
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

	arg := rpc.DeregisterRequest{
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
		s1.config.SerfWANConfig.MemberlistConfig.Port)
	if err := s2.JoinWAN(addr); err != nil {
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
