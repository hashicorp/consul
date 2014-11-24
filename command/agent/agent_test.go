package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
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
	conf.Ports.DNS = 19000 + idx
	conf.Ports.HTTP = 18800 + idx
	conf.Ports.RPC = 18600 + idx
	conf.Ports.SerfLan = 18200 + idx
	conf.Ports.SerfWan = 18400 + idx
	conf.Ports.Server = 18000 + idx
	conf.Server = true
	conf.ACLDatacenter = "dc1"
	conf.ACLMasterToken = "root"

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

	cons.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
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

func makeAgentKeyring(t *testing.T, conf *Config, key string) (string, *Agent) {
	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	conf.DataDir = dir

	fileLAN := filepath.Join(dir, serfLANKeyring)
	if err := initKeyring(fileLAN, key); err != nil {
		t.Fatalf("err: %s", err)
	}
	fileWAN := filepath.Join(dir, serfWANKeyring)
	if err := initKeyring(fileWAN, key); err != nil {
		t.Fatalf("err: %s", err)
	}

	agent, err := Create(conf, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
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
	chk := &CheckType{
		TTL:   time.Minute,
		Notes: "redis health check",
	}
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

	// Ensure the notes are passed through
	if agent.state.Checks()["service:redis"].Notes == "" {
		t.Fatalf("missing redis check notes")
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

	// Remove the consul service
	if err := agent.RemoveService("consul"); err == nil {
		t.Fatalf("should have errored")
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
		Status:  structs.HealthCritical,
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

func TestAgent_AddCheck_MinInterval(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	health := &structs.HealthCheck{
		Node:    "foo",
		CheckID: "mem",
		Name:    "memory util",
		Status:  structs.HealthCritical,
	}
	chk := &CheckType{
		Script:   "exit 0",
		Interval: time.Microsecond,
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
	if mon, ok := agent.checkMonitors["mem"]; !ok {
		t.Fatalf("missing mem monitor")
	} else if mon.Interval != MinInterval {
		t.Fatalf("bad mem monitor interval")
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
		Status:  structs.HealthCritical,
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
		Status:  structs.HealthCritical,
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
	if status.Output != "foo" {
		t.Fatalf("bad: %v", status)
	}
}

func TestAgent_ConsulService(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	testutil.WaitForLeader(t, agent.RPC, "dc1")

	// Consul service is registered
	services := agent.state.Services()
	if _, ok := services[consul.ConsulServiceID]; !ok {
		t.Fatalf("%s service should be registered", consul.ConsulServiceID)
	}

	// Perform anti-entropy on consul service
	if err := agent.state.syncService(consul.ConsulServiceID); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Consul service should be in sync
	if !agent.state.serviceStatus[consul.ConsulServiceID].inSync {
		t.Fatalf("%s service should be in sync", consul.ConsulServiceID)
	}
}

func TestAgent_PersistService(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	if err := agent.AddService(svc, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	file := filepath.Join(agent.config.DataDir, servicesDir, svc.ID)
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	expected, err := json.Marshal(svc)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}
	agent.Shutdown()

	// Should load it back during later start
	agent2, err := Create(config, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer agent2.Shutdown()

	if _, ok := agent2.state.services[svc.ID]; !ok {
		t.Fatalf("bad: %#v", agent2.state.services)
	}

	// Should remove the service file
	if err := agent2.RemoveService(svc.ID); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("err: %s", err)
	}
}

func TestAgent_PersistCheck(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)

	check := &structs.HealthCheck{
		Node:        config.NodeName,
		CheckID:     "service:redis1",
		Name:        "redischeck",
		Status:      structs.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}

	if err := agent.AddCheck(check, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	file := filepath.Join(agent.config.DataDir, checksDir, check.CheckID)
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	expected, err := json.Marshal(check)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	content, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if !bytes.Equal(expected, content) {
		t.Fatalf("bad: %s", string(content))
	}
	agent.Shutdown()

	// Should load it back during later start
	agent2, err := Create(config, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer agent2.Shutdown()

	result, ok := agent2.state.checks[check.CheckID]
	if !ok {
		t.Fatalf("bad: %#v", agent2.state.checks)
	}
	if result.Status != structs.HealthCritical {
		t.Fatalf("bad: %#v", result)
	}

	// Should remove the service file
	if err := agent2.RemoveCheck(check.CheckID); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("err: %s", err)
	}
}
