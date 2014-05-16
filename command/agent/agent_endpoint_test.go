package agent

import (
	"errors"
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
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
		Tags:    []string{"master"},
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

	if int(val[0].Port) != srv.agent.config.Ports.SerfLan {
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

	if int(val[0].Port) != srv.agent.config.Ports.SerfWan {
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

	addr := fmt.Sprintf("127.0.0.1:%d", a2.config.Ports.SerfLan)
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

	addr := fmt.Sprintf("127.0.0.1:%d", a2.config.Ports.SerfWan)
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

	testutil.WaitForResult(func() (bool, error) {
		return len(a2.WANMembers()) == 2, nil
	}, func(err error) {
		t.Fatalf("should have 2 members")
	})
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
	addr := fmt.Sprintf("127.0.0.1:%d", a2.config.Ports.SerfLan)
	_, err := srv.agent.JoinLAN([]string{addr})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	a2.Shutdown()

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

	testutil.WaitForResult(func() (bool, error) {
		m := srv.agent.LANMembers()
		success := m[1].Status == serf.StatusLeft
		return success, errors.New(m[1].Status.String())
	}, func(err error) {
		t.Fatalf("member status is %v, should be left", err)
	})
}

func TestHTTPAgentRegisterCheck(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/check/register", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	args := &CheckDefinition{
		Name: "test",
		CheckType: CheckType{
			TTL: 15 * time.Second,
		},
	}
	req.Body = encodeReq(args)

	obj, err := srv.AgentRegisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	if _, ok := srv.agent.state.Checks()["test"]; !ok {
		t.Fatalf("missing test check")
	}

	if _, ok := srv.agent.checkTTLs["test"]; !ok {
		t.Fatalf("missing test check ttl")
	}
}

func TestHTTPAgentDeregisterCheck(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	if err := srv.agent.AddCheck(chk, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/check/deregister/test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentDeregisterCheck(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	if _, ok := srv.agent.state.Checks()["test"]; ok {
		t.Fatalf("have test check")
	}
}

func TestHTTPAgentPassCheck(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &CheckType{TTL: 15 * time.Second}
	if err := srv.agent.AddCheck(chk, chkType); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/check/pass/test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentCheckPass(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := srv.agent.state.Checks()["test"]
	if state.Status != structs.HealthPassing {
		t.Fatalf("bad: %v", state)
	}
}

func TestHTTPAgentWarnCheck(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &CheckType{TTL: 15 * time.Second}
	if err := srv.agent.AddCheck(chk, chkType); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/check/warn/test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentCheckWarn(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := srv.agent.state.Checks()["test"]
	if state.Status != structs.HealthWarning {
		t.Fatalf("bad: %v", state)
	}
}

func TestHTTPAgentFailCheck(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	chk := &structs.HealthCheck{Name: "test", CheckID: "test"}
	chkType := &CheckType{TTL: 15 * time.Second}
	if err := srv.agent.AddCheck(chk, chkType); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/check/fail/test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentCheckFail(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	state := srv.agent.state.Checks()["test"]
	if state.Status != structs.HealthCritical {
		t.Fatalf("bad: %v", state)
	}
}

func TestHTTPAgentRegisterService(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/service/register", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	args := &ServiceDefinition{
		Name: "test",
		Tags: []string{"master"},
		Port: 8000,
		Check: CheckType{
			TTL: 15 * time.Second,
		},
	}
	req.Body = encodeReq(args)

	obj, err := srv.AgentRegisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure the servie
	if _, ok := srv.agent.state.Services()["test"]; !ok {
		t.Fatalf("missing test service")
	}

	// Ensure we have a check mapping
	if _, ok := srv.agent.state.Checks()["service:test"]; !ok {
		t.Fatalf("missing test check")
	}

	if _, ok := srv.agent.checkTTLs["service:test"]; !ok {
		t.Fatalf("missing test check ttl")
	}
}

func TestHTTPAgentDeregisterService(t *testing.T) {
	dir, srv := makeHTTPServer(t)
	defer os.RemoveAll(dir)
	defer srv.Shutdown()
	defer srv.agent.Shutdown()

	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := srv.agent.AddService(service, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register node
	req, err := http.NewRequest("GET", "/v1/agent/service/deregister/test", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	obj, err := srv.AgentDeregisterService(nil, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if obj != nil {
		t.Fatalf("bad: %v", obj)
	}

	// Ensure we have a check mapping
	if _, ok := srv.agent.state.Services()["test"]; ok {
		t.Fatalf("have test service")
	}

	if _, ok := srv.agent.state.Checks()["test"]; ok {
		t.Fatalf("have test check")
	}
}
