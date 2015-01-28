package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
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

	// Service registration with a single check
	{
		srv := &structs.NodeService{
			ID:      "redis",
			Service: "redis",
			Tags:    []string{"foo"},
			Port:    8000,
		}
		chkTypes := CheckTypes{
			&CheckType{
				TTL:   time.Minute,
				Notes: "redis heath check 2",
			},
		}
		err := agent.AddService(srv, chkTypes, false)
		if err != nil {
			t.Fatalf("err: %v", err)
		}

		// Ensure we have a state mapping
		if _, ok := agent.state.Services()["redis"]; !ok {
			t.Fatalf("missing redis service")
		}

		// Ensure the check is registered
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

	// Service registration with multiple checks
	{
		srv := &structs.NodeService{
			ID:      "memcache",
			Service: "memcache",
			Tags:    []string{"bar"},
			Port:    8000,
		}
		chkTypes := CheckTypes{
			&CheckType{
				TTL:   time.Minute,
				Notes: "memcache health check 1",
			},
			&CheckType{
				TTL:   time.Second,
				Notes: "memcache heath check 2",
			},
		}
		if err := agent.AddService(srv, chkTypes, false); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Ensure we have a state mapping
		if _, ok := agent.state.Services()["memcache"]; !ok {
			t.Fatalf("missing memcache service")
		}

		// Ensure both checks were added
		if _, ok := agent.state.Checks()["service:memcache:1"]; !ok {
			t.Fatalf("missing memcache:1 check")
		}
		if _, ok := agent.state.Checks()["service:memcache:2"]; !ok {
			t.Fatalf("missing memcache:2 check")
		}

		// Ensure a TTL is setup
		if _, ok := agent.checkTTLs["service:memcache:1"]; !ok {
			t.Fatalf("missing memcache:1 check ttl")
		}
		if _, ok := agent.checkTTLs["service:memcache:2"]; !ok {
			t.Fatalf("missing memcache:2 check ttl")
		}

		// Ensure the notes are passed through
		if agent.state.Checks()["service:memcache:1"].Notes == "" {
			t.Fatalf("missing redis check notes")
		}
		if agent.state.Checks()["service:memcache:2"].Notes == "" {
			t.Fatalf("missing redis check notes")
		}
	}
}

func TestAgent_RemoveService(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	// Remove a service that doesn't exist
	if err := agent.RemoveService("redis", false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove the consul service
	if err := agent.RemoveService("consul", false); err == nil {
		t.Fatalf("should have errored")
	}

	// Remove without an ID
	if err := agent.RemoveService("", false); err == nil {
		t.Fatalf("should have errored")
	}

	// Removing a service with a single check works
	{
		srv := &structs.NodeService{
			ID:      "memcache",
			Service: "memcache",
			Port:    8000,
		}
		chkTypes := CheckTypes{&CheckType{TTL: time.Minute}}

		if err := agent.AddService(srv, chkTypes, false); err != nil {
			t.Fatalf("err: %v", err)
		}

		if err := agent.RemoveService("memcache", false); err != nil {
			t.Fatalf("err: %s", err)
		}
		if _, ok := agent.state.Checks()["service:memcache"]; ok {
			t.Fatalf("have memcache check")
		}
	}

	// Removing a service with multiple checks works
	{
		srv := &structs.NodeService{
			ID:      "redis",
			Service: "redis",
			Port:    8000,
		}
		chkTypes := CheckTypes{
			&CheckType{TTL: time.Minute},
			&CheckType{TTL: 30 * time.Second},
		}
		if err := agent.AddService(srv, chkTypes, false); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Remove the service
		if err := agent.RemoveService("redis", false); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Ensure we have a state mapping
		if _, ok := agent.state.Services()["redis"]; ok {
			t.Fatalf("have redis service")
		}

		// Ensure checks were removed
		if _, ok := agent.state.Checks()["service:redis:1"]; ok {
			t.Fatalf("check redis:1 should be removed")
		}
		if _, ok := agent.state.Checks()["service:redis:2"]; ok {
			t.Fatalf("check redis:2 should be removed")
		}

		// Ensure a TTL is setup
		if _, ok := agent.checkTTLs["service:redis:1"]; ok {
			t.Fatalf("check ttl for redis:1 should be removed")
		}
		if _, ok := agent.checkTTLs["service:redis:2"]; ok {
			t.Fatalf("check ttl for redis:2 should be removed")
		}
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
	err := agent.AddCheck(health, chk, false)
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
	err := agent.AddCheck(health, chk, false)
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

func TestAgent_AddCheck_MissingService(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	health := &structs.HealthCheck{
		Node:      "foo",
		CheckID:   "baz",
		Name:      "baz check 1",
		ServiceID: "baz",
	}
	chk := &CheckType{
		Script:   "exit 0",
		Interval: time.Microsecond,
	}
	err := agent.AddCheck(health, chk, false)
	if err == nil || err.Error() != `ServiceID "baz" does not exist` {
		t.Fatalf("expected service id error, got: %v", err)
	}
}

func TestAgent_RemoveCheck(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	// Remove check that doesn't exist
	if err := agent.RemoveCheck("mem", false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove without an ID
	if err := agent.RemoveCheck("", false); err == nil {
		t.Fatalf("should have errored")
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
	err := agent.AddCheck(health, chk, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Remove check
	if err := agent.RemoveCheck("mem", false); err != nil {
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
	err := agent.AddCheck(health, chk, false)
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
	config.Server = false
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(agent.config.DataDir, servicesDir, stringHash(svc.ID))

	// Check is not persisted unless requested
	if err := agent.AddService(svc, nil, false); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Persists to file if requested
	if err := agent.AddService(svc, nil, true); err != nil {
		t.Fatalf("err: %v", err)
	}

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
}

func TestAgent_PurgeService(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	file := filepath.Join(agent.config.DataDir, servicesDir, stringHash(svc.ID))
	if err := agent.AddService(svc, nil, true); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Not removed
	if err := agent.RemoveService(svc.ID, false); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed
	if err := agent.RemoveService(svc.ID, true); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("bad: %#v", err)
	}
}

func TestAgent_PurgeServiceOnDuplicate(t *testing.T) {
	config := nextConfig()
	config.Server = false
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	svc1 := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// First persist the service
	if err := agent.AddService(svc1, nil, true); err != nil {
		t.Fatalf("err: %v", err)
	}
	agent.Shutdown()

	// Try bringing the agent back up with the service already
	// existing in the config
	svc2 := &ServiceDefinition{
		ID:   "redis",
		Name: "redis",
		Tags: []string{"bar"},
		Port: 9000,
	}

	config.Services = []*ServiceDefinition{svc2}
	agent2, err := Create(config, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer agent2.Shutdown()

	file := filepath.Join(agent.config.DataDir, servicesDir, stringHash(svc1.ID))
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted service")
	}
	result, ok := agent2.state.services[svc2.ID]
	if !ok {
		t.Fatalf("missing service registration")
	}
	if !reflect.DeepEqual(result.Tags, svc2.Tags) || result.Port != svc2.Port {
		t.Fatalf("bad: %#v", result)
	}
}

func TestAgent_PersistCheck(t *testing.T) {
	config := nextConfig()
	config.Server = false
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	check := &structs.HealthCheck{
		Node:    config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  structs.HealthPassing,
	}
	chkType := &CheckType{
		Script:   "/bin/true",
		Interval: 10 * time.Second,
	}

	file := filepath.Join(agent.config.DataDir, checksDir, stringHash(check.CheckID))

	// Not persisted if not requested
	if err := agent.AddCheck(check, chkType, false); err != nil {
		t.Fatalf("err: %v", err)
	}
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should not persist")
	}

	// Should persist if requested
	if err := agent.AddCheck(check, chkType, true); err != nil {
		t.Fatalf("err: %v", err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	p := persistedCheck{check, chkType}
	expected, err := json.Marshal(p)
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

	result, ok := agent2.state.checks[p.Check.CheckID]
	if !ok {
		t.Fatalf("bad: %#v", agent2.state.checks)
	}
	if result.Status != structs.HealthCritical {
		t.Fatalf("bad: %#v", result)
	}

	// Should have restored the monitor
	if _, ok := agent2.checkMonitors[p.Check.CheckID]; !ok {
		t.Fatalf("bad: %#v", agent2.checkMonitors)
	}
}

func TestAgent_PurgeCheck(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	check := &structs.HealthCheck{
		Node:    config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  structs.HealthPassing,
	}

	file := filepath.Join(agent.config.DataDir, checksDir, stringHash(check.CheckID))
	if err := agent.AddCheck(check, nil, true); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Not removed
	if err := agent.RemoveCheck(check.CheckID, false); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Removed
	if err := agent.RemoveCheck(check.CheckID, true); err != nil {
		t.Fatalf("err: %s", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatalf("bad: %#v", err)
	}
}

func TestAgent_PurgeCheckOnDuplicate(t *testing.T) {
	config := nextConfig()
	config.Server = false
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	check1 := &structs.HealthCheck{
		Node:    config.NodeName,
		CheckID: "mem",
		Name:    "memory check",
		Status:  structs.HealthPassing,
	}

	// First persist the check
	if err := agent.AddCheck(check1, nil, true); err != nil {
		t.Fatalf("err: %v", err)
	}
	agent.Shutdown()

	// Start again with the check registered in config
	check2 := &CheckDefinition{
		ID:    "mem",
		Name:  "memory check",
		Notes: "my cool notes",
		CheckType: CheckType{
			Script:   "/bin/check-redis.py",
			Interval: 30 * time.Second,
		},
	}

	config.Checks = []*CheckDefinition{check2}
	agent2, err := Create(config, nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer agent2.Shutdown()

	file := filepath.Join(agent.config.DataDir, checksDir, stringHash(check1.CheckID))
	if _, err := os.Stat(file); err == nil {
		t.Fatalf("should have removed persisted check")
	}
	result, ok := agent2.state.checks[check2.ID]
	if !ok {
		t.Fatalf("missing check registration")
	}
	expected := check2.HealthCheck(config.NodeName)
	if !reflect.DeepEqual(expected, result) {
		t.Fatalf("bad: %#v", result)
	}
}

func TestAgent_unloadChecks(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	// First register a service
	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}
	if err := agent.AddService(svc, nil, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Register a check
	check1 := &structs.HealthCheck{
		Node:        config.NodeName,
		CheckID:     "service:redis",
		Name:        "redischeck",
		Status:      structs.HealthPassing,
		ServiceID:   "redis",
		ServiceName: "redis",
	}
	if err := agent.AddCheck(check1, nil, false); err != nil {
		t.Fatalf("err: %s", err)
	}
	found := false
	for check, _ := range agent.state.Checks() {
		if check == check1.CheckID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("check should have been registered")
	}

	// Unload all of the checks
	if err := agent.unloadChecks(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure it was unloaded
	for check, _ := range agent.state.Checks() {
		if check == check1.CheckID {
			t.Fatalf("should have unloaded checks")
		}
	}
}

func TestAgent_unloadServices(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := agent.AddService(svc, nil, false); err != nil {
		t.Fatalf("err: %v", err)
	}
	found := false
	for id, _ := range agent.state.Services() {
		if id == svc.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("should have registered service")
	}

	// Unload all services
	if err := agent.unloadServices(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure it was unloaded and the consul service remains
	found = false
	for id, _ := range agent.state.Services() {
		if id == svc.ID {
			t.Fatalf("should have unloaded services")
		}
		if id == consul.ConsulServiceID {
			found = true
		}
	}
	if !found {
		t.Fatalf("consul service should not be removed")
	}
}

func TestAgent_ServiceMaintenanceMode(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	svc := &structs.NodeService{
		ID:      "redis",
		Service: "redis",
		Tags:    []string{"foo"},
		Port:    8000,
	}

	// Register the service
	if err := agent.AddService(svc, nil, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Enter maintenance mode for the service
	if err := agent.EnableServiceMaintenance("redis", "broken"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the critical health check was added
	checkID := serviceMaintCheckID("redis")
	check, ok := agent.state.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered critical maintenance check")
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	if err := agent.DisableServiceMaintenance("redis"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was deregistered
	if _, ok := agent.state.Checks()[checkID]; ok {
		t.Fatalf("should have deregistered maintenance check")
	}

	// Enter service maintenance mode without providing a reason
	if err := agent.EnableServiceMaintenance("redis", ""); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Ensure the check was registered with the default notes
	check, ok = agent.state.Checks()[checkID]
	if !ok {
		t.Fatalf("should have registered critical check")
	}
	if check.Notes != defaultServiceMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}

func TestAgent_NodeMaintenanceMode(t *testing.T) {
	config := nextConfig()
	dir, agent := makeAgent(t, config)
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	// Enter maintenance mode for the node
	agent.EnableNodeMaintenance("broken")

	// Make sure the critical health check was added
	check, ok := agent.state.Checks()[nodeMaintCheckID]
	if !ok {
		t.Fatalf("should have registered critical node check")
	}

	// Ensure the reason was set in notes
	if check.Notes != "broken" {
		t.Fatalf("bad: %#v", check)
	}

	// Leave maintenance mode
	agent.DisableNodeMaintenance()

	// Ensure the check was deregistered
	if _, ok := agent.state.Checks()[nodeMaintCheckID]; ok {
		t.Fatalf("should have deregistered critical node check")
	}

	// Enter maintenance mode without passing a reason
	agent.EnableNodeMaintenance("")

	// Make sure the check was registered with the default note
	check, ok = agent.state.Checks()[nodeMaintCheckID]
	if !ok {
		t.Fatalf("should have registered critical node check")
	}
	if check.Notes != defaultNodeMaintReason {
		t.Fatalf("bad: %#v", check)
	}
}
