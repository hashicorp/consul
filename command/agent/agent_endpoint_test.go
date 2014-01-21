package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestHTTPAgentServices(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	srv1 := &structs.NodeService{
		ID:      "mysql",
		Service: "mysql",
		Tag:     "master",
		Port:    5000,
	}
	srv.agent.state.AddService(srv1)

	obj, err := srv.AgentServices(nil, nil)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[string]*structs.NodeService)
	if len(val) != 1 {
		t.Fatalf("bad services: %v", obj)
	}
	if val["mysql"].Port != 5000 {
		t.Fatalf("bad service: %v", obj)
	}
}

func TestHTTPAgentChecks(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	chk1 := &structs.HealthCheck{
		Node:    srv.agent.config.NodeName,
		CheckID: "mysql",
		Name:    "mysql",
		Status:  structs.HealthPassing,
	}
	srv.agent.state.AddCheck(chk1)

	obj, err := srv.AgentChecks(nil, nil)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.(map[string]*structs.HealthCheck)
	if len(val) != 1 {
		t.Fatalf("bad checks: %v", obj)
	}
	if val["mysql"].Status != structs.HealthPassing {
		t.Fatalf("bad check: %v", obj)
	}
}

func TestHTTPAgentMembers(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	req, err := http.NewRequest("GET", "/v1/agent/members", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentMembers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.([]serf.Member)
	if len(val) == 0 {
		t.Fatalf("bad members: %v", obj)
	}

	if int(val[0].Port) != srv.agent.config.SerfLanPort {
		t.Fatalf("not lan: %v", obj)
	}
}

func TestHTTPAgentMembers_WAN(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	req, err := http.NewRequest("GET", "/v1/agent/members?wan=true", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentMembers(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	val := obj.([]serf.Member)
	if len(val) == 0 {
		t.Fatalf("bad members: %v", obj)
	}

	if int(val[0].Port) != srv.agent.config.SerfWanPort {
		t.Fatalf("not wan: %v", obj)
	}
}

func TestHTTPAgentJoin(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	dir2, a2 := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir2)
	defer a2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d", a2.config.SerfLanPort)
	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/agent/join/%s", addr), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentJoin(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

	if len(a2.LANMembers()) != 2 {
		t.Fatalf("should have 2 members")
	}
}

func TestHTTPAgentJoin_WAN(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	dir2, a2 := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir2)
	defer a2.Shutdown()

	addr := fmt.Sprintf("127.0.0.1:%d", a2.config.SerfWanPort)
	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/agent/join/%s?wan=true", addr), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentJoin(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

	if len(a2.WANMembers()) != 2 {
		t.Fatalf("should have 2 members")
	}
}

func TestHTTPAgentForceLeave(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	dir2, a2 := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir2)
	defer a2.Shutdown()

	// Join first
	addr := fmt.Sprintf("127.0.0.1:%d", a2.config.SerfLanPort)
	_, err := srv.agent.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Shutdown, wait for detection
	a2.Shutdown()
	time.Sleep(500 * time.Millisecond)

	// Force leave now
	req, err := http.NewRequest("GET", fmt.Sprintf("/v1/agent/force-leave/%s", a2.config.NodeName), nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentForceLeave(nil, req)
	if err != nil {
		t.Fatalf("Err: %v", err)
	}
	if obj != nil {
		t.Fatalf("Err: %v", obj)
	}

	// SHould be left
	mem := srv.agent.LANMembers()
	if mem[1].Status != serf.StatusLeft {
		t.Fatalf("should have left: %v", mem)
	}
}
