package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul"
	"io/ioutil"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var offset uint64

func nextConfig() *Config {
	idx := atomic.AddUint64(&offset, 1)
	conf := DefaultConfig()

	conf.Bootstrap = true
	conf.Datacenter = "dc1"
	conf.HTTPAddr = fmt.Sprintf("127.0.0.1:%d", 8500+10*idx)
	conf.RPCAddr = fmt.Sprintf("127.0.0.1:%d", 8400+10*idx)
	conf.SerfBindAddr = "127.0.0.1"
	conf.SerfLanPort = int(8301 + 10*idx)
	conf.SerfWanPort = int(8302 + 10*idx)
	conf.Server = true

	cons := consul.DefaultConfig()
	conf.ConsulConfig = cons

	cons.SerfLANConfig.MemberlistConfig.ProbeTimeout = 200 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.ProbeInterval = time.Second
	cons.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.SerfWANConfig.MemberlistConfig.ProbeTimeout = 200 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.ProbeInterval = time.Second
	cons.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	cons.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	return conf
}

func makeAgent(t *testing.T, conf *Config) (string, *Agent) {
	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf(fmt.Sprintf("err: %v", err))
	}

	conf.DataDir = dir
	agent, err := Create(conf, nil)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf(fmt.Sprintf("err: %v", err))
	}

	return dir, agent
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
