package command

import (
	"fmt"
	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/consul"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

var offset uint64

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())
}

type agentWrapper struct {
	dir    string
	config *agent.Config
	agent  *agent.Agent
	rpc    *agent.AgentRPC
	addr   string
}

func (a *agentWrapper) Shutdown() {
	a.rpc.Shutdown()
	a.agent.Shutdown()
	os.RemoveAll(a.dir)
}

func testAgent(t *testing.T) *agentWrapper {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	lw := agent.NewLogWriter(512)
	mult := io.MultiWriter(os.Stderr, lw)

	conf := nextConfig()

	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf(fmt.Sprintf("err: %v", err))
	}
	conf.DataDir = dir

	a, err := agent.Create(conf, lw)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf(fmt.Sprintf("err: %v", err))
	}

	rpc := agent.NewAgentRPC(a, l, mult, lw)
	return &agentWrapper{
		dir:    dir,
		config: conf,
		agent:  a,
		rpc:    rpc,
		addr:   l.Addr().String(),
	}
}

func nextConfig() *agent.Config {
	idx := int(atomic.AddUint64(&offset, 1))
	conf := agent.DefaultConfig()

	conf.Bootstrap = true
	conf.Datacenter = "dc1"
	conf.NodeName = fmt.Sprintf("Node %d", idx)
	conf.BindAddr = "127.0.0.1"
	conf.Server = true

	conf.Ports.HTTP = 10000 + 10*idx
	conf.Ports.RPC = 10100 + 10*idx
	conf.Ports.SerfLan = 10201 + 10*idx
	conf.Ports.SerfWan = 10202 + 10*idx
	conf.Ports.Server = 10300 + 10*idx

	cons := consul.DefaultConfig()
	conf.ConsulConfig = cons

	cons.SerfLANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.SerfWANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	cons.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	return conf
}
