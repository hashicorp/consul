package consul

import (
	"github.com/hashicorp/consul/rpc"
	"testing"
)

func TestFSM_RegisterNode(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := rpc.RegisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
		Address:    "127.0.0.1",
	}
	buf, err := rpc.Encode(rpc.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service registered
	services := fsm.state.NodeServices("foo")
	if len(services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}

func TestFSM_RegisterNode_Service(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := rpc.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
	}
	buf, err := rpc.Encode(rpc.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service registered
	services := fsm.state.NodeServices("foo")
	if _, ok := services["db"]; !ok {
		t.Fatalf("not registered!")
	}
}

func TestFSM_DeregisterService(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := rpc.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
	}
	buf, err := rpc.Encode(rpc.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	dereg := rpc.DeregisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		ServiceName: "db",
	}
	buf, err = rpc.Encode(rpc.DeregisterRequestType, dereg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); !found {
		t.Fatalf("not found!")
	}

	// Verify service not registered
	services := fsm.state.NodeServices("foo")
	if _, ok := services["db"]; ok {
		t.Fatalf("db registered!")
	}
}

func TestFSM_DeregisterNode(t *testing.T) {
	fsm, err := NewFSM()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	req := rpc.RegisterRequest{
		Datacenter:  "dc1",
		Node:        "foo",
		Address:     "127.0.0.1",
		ServiceName: "db",
		ServiceTag:  "master",
		ServicePort: 8000,
	}
	buf, err := rpc.Encode(rpc.RegisterRequestType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp := fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	dereg := rpc.DeregisterRequest{
		Datacenter: "dc1",
		Node:       "foo",
	}
	buf, err = rpc.Encode(rpc.DeregisterRequestType, dereg)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	resp = fsm.Apply(buf)
	if resp != nil {
		t.Fatalf("resp: %v", resp)
	}

	// Verify we are registered
	if found, _ := fsm.state.GetNode("foo"); found {
		t.Fatalf("found!")
	}

	// Verify service not registered
	services := fsm.state.NodeServices("foo")
	if len(services) != 0 {
		t.Fatalf("Services: %v", services)
	}
}
