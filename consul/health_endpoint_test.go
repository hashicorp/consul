package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"os"
	"testing"
	"time"
)

func TestHealth_ChecksInState(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Name:   "memory utilization",
			Status: structs.HealthPassing,
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var checks structs.HealthChecks
	inState := structs.ChecksInStateRequest{
		Datacenter: "dc1",
		State:      structs.HealthPassing,
	}
	if err := client.Call("Health.ChecksInState", &inState, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Name != "memory utilization" {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_NodeChecks(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Check: &structs.HealthCheck{
			Name:   "memory utilization",
			Status: structs.HealthPassing,
		},
	}
	var out struct{}
	if err := client.Call("Catalog.Register", &arg, &out); err != nil {
		t.Fatalf("err: %v", err)
	}

	var checks structs.HealthChecks
	node := structs.NodeSpecificRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	if err := client.Call("Health.NodeChecks", &node, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Name != "memory utilization" {
		t.Fatalf("Bad: %v", checks)
	}
}

func TestHealth_ServiceChecks(t *testing.T) {
	dir1, s1 := testServer(t)
	defer os.RemoveAll(dir1)
	defer s1.Shutdown()
	client := rpcClient(t, s1)
	defer client.Close()

	// Wait for leader
	time.Sleep(100 * time.Millisecond)

	arg := structs.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
		Service: &structs.NodeService{
			ID:      "db",
			Service: "db",
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

	var checks structs.HealthChecks
	node := structs.ServiceSpecificRequest{
		Datacenter:  "dc1",
		ServiceName: "db",
	}
	if err := client.Call("Health.ServiceChecks", &node, &checks); err != nil {
		t.Fatalf("err: %v", err)
	}

	if len(checks) != 1 {
		t.Fatalf("Bad: %v", checks)
	}
	if checks[0].Name != "db connect" {
		t.Fatalf("Bad: %v", checks)
	}
}
