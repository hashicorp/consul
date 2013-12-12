package consul

import (
	"fmt"
	"github.com/hashicorp/consul/rpc"
	"os"
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
