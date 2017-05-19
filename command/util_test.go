package command

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/version"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
)

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	version.Version = "0.8.0"
}

type server struct {
	agent    *agent.Agent
	config   *agent.Config
	httpAddr string
	dir      string
}

func (a *server) Shutdown() {
	a.agent.Shutdown()
	os.RemoveAll(a.dir)
}

func testAgent(t *testing.T) *server {
	return testAgentWithConfig(t, nil)
}

func testAgentWithAPIClient(t *testing.T) (*server, *api.Client) {
	agent := testAgentWithConfig(t, func(c *agent.Config) {})
	client, err := api.NewClient(&api.Config{Address: agent.httpAddr})
	if err != nil {
		t.Fatalf("consul client: %#v", err)
	}
	return agent, client
}

func testAgentWithConfig(t *testing.T, cb func(c *agent.Config)) *server {
	conf := nextConfig()
	if cb != nil {
		cb(conf)
	}

	conf.DataDir = testutil.TempDir(t, "agent")
	a, err := agent.NewAgent(conf)
	if err != nil {
		os.RemoveAll(conf.DataDir)
		t.Fatal("Error creating agent:", err)
	}
	a.LogOutput = logger.NewLogWriter(512)
	if err := a.Start(); err != nil {
		os.RemoveAll(conf.DataDir)
		t.Fatalf("Error starting agent: %v", err)
	}

	conf.Addresses.HTTP = "127.0.0.1"
	addr := fmt.Sprintf("%s:%d", conf.Addresses.HTTP, conf.Ports.HTTP)
	return &server{agent: a, config: conf, httpAddr: addr, dir: conf.DataDir}
}

var nextPort uint64 = 10000

func nextConfig() *agent.Config {
	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	port := int(atomic.AddUint64(&nextPort, 10))

	conf := agent.DefaultConfig()
	conf.Bootstrap = true
	conf.Datacenter = "dc1"
	conf.NodeName = fmt.Sprintf("Node %d", port)
	conf.NodeID = types.NodeID(nodeID)
	conf.BindAddr = "127.0.0.1"
	conf.Server = true
	conf.Version = version.Version
	conf.Ports = agent.PortConfig{
		DNS:     port + 1,
		HTTP:    port + 2,
		HTTPS:   port + 3,
		SerfLan: port + 4,
		SerfWan: port + 5,
		Server:  port + 6,
	}

	cons := consul.DefaultConfig()
	conf.ConsulConfig = cons

	cons.SerfLANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	cons.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.SerfWANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	cons.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	cons.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
	cons.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	cons.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	return conf
}

func assertNoTabs(t *testing.T, c cli.Command) {
	if strings.ContainsRune(c.Help(), '\t') {
		t.Errorf("%#v help output contains tabs", c)
	}
}
