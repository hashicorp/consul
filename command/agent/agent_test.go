package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"io"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var offset uint64

func nextConfig() *Config {
	idx := int(atomic.AddUint64(&offset, 1))
	conf := DefaultConfig()

	conf.AdvertiseAddr = "127.0.0.1"
	conf.Bootstrap = true
	conf.Datacenter = "dc1"
	conf.NodeName = fmt.Sprintf("Node %d", idx)
	conf.BindAddr = "127.0.0.1"
	conf.Ports.DNS = 18600 + idx
	conf.Ports.HTTP = 18500 + idx
	conf.Ports.RPC = 18400 + idx
	conf.Ports.SerfLan = 18200 + idx
	conf.Ports.SerfWan = 18300 + idx
	conf.Ports.Server = 18100 + idx
	conf.Server = true

	cons := consul.DefaultConfig()
	conf.ConsulConfig = cons

	cons.SerfLANConfig.MemberlistConfig.SuspicionMult = 3
	cons.SerfLANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.SerfWANConfig.MemberlistConfig.SuspicionMult = 3
	cons.SerfWANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	cons.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	return conf
}

func makeAgentLog(t *testing.T, conf *Config, l io.Writer) (string, *Agent) {
	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf(fmt.Sprintf("err: %v", err))
	}

	conf.DataDir = dir
	agent, err := Create(conf, l)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf(fmt.Sprintf("err: %v", err))
	}

	return dir, agent
}

func makeAgent(t *testing.T, conf *Config) (string, *Agent) {
	return makeAgentLog(t, conf, nil)
}

func TestAgentStartStop(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	if err := agent.Leave(); err != nil {
		t.Fatalf("err: %v", err)
	}
	if err := agent.Shutdown(); err != nil {
		t.Fatalf("err: %v", err)
	}

	select {
	case <-agent.ShutdownCh():
	default:
		t.Fatalf("should be closed")
	}
}

func TestAgent_RPCPing(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	var out struct{}
	if err := agent.RPC("Status.Ping", struct{}{}, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAgent_AddService(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	srv := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	chk := &CheckType{TTL: time.Minute}
	err := agent.AddService(srv, chk)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a state mapping
	if _, ok := agent.state.Services()["redis"]; !ok {
		t.Fatalf("missing redis service")
	}

	// Ensure we have a check mapping
	if _, ok := agent.state.Checks()["service:redis"]; !ok {
		t.Fatalf("missing redis check")
	}

	// Ensure a TTL is setup
	if _, ok := agent.checkTTLs["service:redis"]; !ok {
		t.Fatalf("missing redis check ttl")
	}
}

func TestAgent_RemoveService(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	// Remove a service that doesn't exist
	if err := agent.RemoveService("redis"); err != nil {
		t.Fatalf("err: %v", err)
	}

	srv := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Port:    8000,
	}
	chk := &CheckType{TTL: time.Minute}
	if err := agent.AddService(srv, chk); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove the service
	if err := agent.RemoveService("redis"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a state mapping
	if _, ok := agent.state.Services()["redis"]; ok {
		t.Fatalf("have redis service")
	}

	// Ensure we have a check mapping
	if _, ok := agent.state.Checks()["service:redis"]; ok {
		t.Fatalf("have redis check")
	}

	// Ensure a TTL is setup
	if _, ok := agent.checkTTLs["service:redis"]; ok {
		t.Fatalf("have redis check ttl")
	}
}

func TestAgent_AddCheck(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  structs.HealthUnknown,
	}
	chk := &CheckType{
		Script:   "exit 0",
		Interval: 15 * time.Second,
	}
	err := agent.AddCheck(health, chk)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	if _, ok := agent.state.Checks()["mem"]; !ok {
		t.Fatalf("missing mem check")
	}

	// Ensure a TTL is setup
	if _, ok := agent.checkMonitors["mem"]; !ok {
		t.Fatalf("missing mem monitor")
	}
}

func TestAgent_RemoveCheck(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	// Remove check that doesn't exist
	if err := agent.RemoveCheck("mem"); err != nil {
		t.Fatalf("err: %v", err)
	}

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  structs.HealthUnknown,
	}
	chk := &CheckType{
		Script:   "exit 0",
		Interval: 15 * time.Second,
	}
	err := agent.AddCheck(health, chk)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove check
	if err := agent.RemoveCheck("mem"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	if _, ok := agent.state.Checks()["mem"]; ok {
		t.Fatalf("have mem check")
	}

	// Ensure a TTL is setup
	if _, ok := agent.checkMonitors["mem"]; ok {
		t.Fatalf("have mem monitor")
	}
}

func TestAgent_UpdateCheck(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  structs.HealthUnknown,
	}
	chk := &CheckType{
		TTL: 15 * time.Second,
	}
	err := agent.AddCheck(health, chk)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove check
	if err := agent.UpdateCheck("mem", structs.HealthPassing, "foo"); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Ensure we have a check mapping
	status := agent.state.Checks()["mem"]
	if status.Status != structs.HealthPassing {
		t.Fatalf("bad: %v", status)
	}
	if status.Notes != "foo" {
		t.Fatalf("bad: %v", status)
	}
}
