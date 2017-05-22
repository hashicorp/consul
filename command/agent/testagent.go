package agent

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/consul/version"
	uuid "github.com/hashicorp/go-uuid"
)

func init() {
	rand.Seed(time.Now().UnixNano()) // seed random number generator
}

// TempDir defines the base dir for temporary directories.
var TempDir = os.TempDir()

// TestAgent encapsulates an Agent with a default configuration and
// startup procedure suitable for testing. It panics if there are errors
// during creation or startup instead of returning errors. It manages a
// temporary data directory which is removed after shutdown.
//
// todo(fs): do we need the temp data dir if we run in dev mode?
type TestAgent struct {
	// Name is an optional name of the agent.
	Name string

	// Config is the agent configuration. If Config is nil then
	// TestConfig() is used. If Config.DataDir is set then it is
	// the callers responsibility to clean up the data directory.
	// Otherwise, a temporary data directory is created and removed
	// when Shutdown() is called.
	Config *Config

	// LogOutput is the sink for the logs. If nil, logs are written
	// to os.Stderr.
	LogOutput io.Writer

	// LogWriter is used for streaming logs.
	LogWriter *logger.LogWriter

	// DataDir is the data directory which is used when Config.DataDir
	// is not set. It is created automatically and removed when
	// Shutdown() is called.
	DataDir string

	// Key is the optional encryption key for the LAN and WAN keyring.
	Key string

	// dns is a reference to the first started DNS endpoint.
	// It is valid after Start().
	dns *DNSServer

	// srv is a reference to the first started HTTP endpoint.
	// It is valid after Start().
	srv *HTTPServer

	// Agent is the embedded consul agent.
	// It is valid after Start().
	*Agent
}

// NewTestAgent returns a started agent with the given name and
// configuration. It panics if the agent could not be started. The
// caller should call Shutdown() to stop the agent and remove temporary
// directories.
func NewTestAgent(name string, c *Config) *TestAgent {
	a := &TestAgent{Name: name, Config: c}
	a.Start()
	return a
}

type panicFailer struct{}

func (f *panicFailer) Log(args ...interface{}) { fmt.Println(args...) }
func (f *panicFailer) FailNow()                { panic("failed") }

// Start starts a test agent. It panics if the agent could not be started.
func (a *TestAgent) Start() *TestAgent {
	if a.Agent != nil {
		panic("TestAgent already started")
	}
	if a.Config == nil {
		a.Config = TestConfig()
	}
	if a.Config.DataDir == "" {
		name := "agent"
		if a.Name != "" {
			name = a.Name + "-agent"
		}
		name = strings.Replace(name, "/", "_", -1)
		d, err := ioutil.TempDir(TempDir, name)
		if err != nil {
			panic(fmt.Sprintf("Error creating data dir %s: %s", filepath.Join(TempDir, name), err))
		}
		a.DataDir = d
		a.Config.DataDir = d
	}
	if a.Config.DNSRecursor != "" {
		a.Config.DNSRecursors = append(a.Config.DNSRecursors, a.Config.DNSRecursor)
	}
	if a.Key != "" {
		writeKey := func(key, filename string) {
			path := filepath.Join(a.Config.DataDir, filename)
			if err := initKeyring(path, key); err != nil {
				panic(fmt.Sprintf("Error creating keyring %s: %s", path, err))
			}
		}
		writeKey(a.Key, serfLANKeyring)
		writeKey(a.Key, serfWANKeyring)
	}

	agent, err := NewAgent(a.Config)
	if err != nil {
		panic(fmt.Sprintf("Error creating agent: %s", err))
	}
	a.Agent = agent
	a.Agent.LogOutput = a.LogOutput
	a.Agent.LogWriter = a.LogWriter
	tenTimes := &retry.Counter{Count: 10, Wait: 100 * time.Millisecond}
	retry.RunWith(tenTimes, &panicFailer{}, func(r *retry.R) {
		err := a.Agent.Start()
		if err == nil {
			return
		}

		// retry with different ports on port conflict
		if strings.Contains(err.Error(), "bind: address already in use") {
			pickRandomPorts(a.Config)
			r.Fatal("port conflict")
		}

		// do not retry on other failures
		panic(fmt.Sprintf("Error starting agent: %s", err))
	})

	var out structs.IndexedNodes
	retry.Run(&panicFailer{}, func(r *retry.R) {
		if len(a.httpServers) == 0 {
			r.Fatal("waiting for server")
		}
		if a.Config.Bootstrap && a.Config.Server {
			// Ensure we have a leader and a node registration.
			args := &structs.DCSpecificRequest{Datacenter: a.Config.Datacenter}
			if err := a.RPC("Catalog.ListNodes", args, &out); err != nil {
				r.Fatalf("Catalog.ListNodes failed: %v", err)
			}
			if !out.QueryMeta.KnownLeader {
				r.Fatalf("No leader")
			}
			if out.Index == 0 {
				r.Fatalf("Consul index is 0")
			}
		} else {
			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.httpServers[0].AgentSelf(resp, req)
			if err != nil || resp.Code != 200 {
				r.Fatal("failed OK respose", err)
			}
		}
	})
	a.dns = a.dnsServers[0]
	a.srv = a.httpServers[0]
	return a
}

// Shutdown stops the agent and removes the data directory if it is
// managed by the test agent.
func (a *TestAgent) Shutdown() error {
	defer func() {
		if a.DataDir != "" {
			os.RemoveAll(a.DataDir)
		}
	}()
	return a.Agent.Shutdown()
}

func (a *TestAgent) consulConfig() *consul.Config {
	c, err := a.Agent.consulConfig()
	if err != nil {
		panic(err)
	}
	return c
}

// pickRandomPorts selects random ports from fixed size random blocks of
// ports. This does not eliminate the chance for port conflict but
// reduces it significanltly with little overhead. Furthermore, asking
// the kernel for a random port by binding to port 0 prolongs the test
// execution (in our case +20sec) while also not fully eliminating the
// chance of port conflicts for concurrently executed test binaries.
// Instead of relying on one set of ports to be sufficient we retry
// starting the agent with different ports on port conflict.
func pickRandomPorts(c *Config) {
	port := 1030 + int(rand.Int31n(64400))
	c.Ports.DNS = port + 1
	c.Ports.HTTP = port + 2
	c.Ports.SerfLan = port + 3
	c.Ports.SerfWan = port + 4
	c.Ports.Server = port + 5
}

// BoolTrue and BoolFalse exist to create a *bool value.
var BoolTrue = true
var BoolFalse = false

// TestConfig returns a unique default configuration for testing an
// agent.
func TestConfig() *Config {
	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	cfg := DefaultConfig()
	pickRandomPorts(cfg)

	cfg.Version = version.Version
	cfg.VersionPrerelease = "c.d"

	cfg.NodeID = types.NodeID(nodeID)
	cfg.NodeName = "Node " + nodeID
	cfg.BindAddr = "127.0.0.1"
	cfg.AdvertiseAddr = "127.0.0.1"
	cfg.Datacenter = "dc1"
	cfg.Bootstrap = true
	cfg.Server = true
	cfg.ACLEnforceVersion8 = &BoolFalse
	cfg.ACLDatacenter = cfg.Datacenter
	cfg.ACLMasterToken = "root"

	ccfg := consul.DefaultConfig()
	cfg.ConsulConfig = ccfg

	ccfg.SerfLANConfig.MemberlistConfig.SuspicionMult = 3
	ccfg.SerfLANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	ccfg.SerfLANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	ccfg.SerfLANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	ccfg.SerfWANConfig.MemberlistConfig.SuspicionMult = 3
	ccfg.SerfWANConfig.MemberlistConfig.ProbeTimeout = 100 * time.Millisecond
	ccfg.SerfWANConfig.MemberlistConfig.ProbeInterval = 100 * time.Millisecond
	ccfg.SerfWANConfig.MemberlistConfig.GossipInterval = 100 * time.Millisecond

	ccfg.RaftConfig.LeaderLeaseTimeout = 20 * time.Millisecond
	ccfg.RaftConfig.HeartbeatTimeout = 40 * time.Millisecond
	ccfg.RaftConfig.ElectionTimeout = 40 * time.Millisecond

	ccfg.CoordinateUpdatePeriod = 100 * time.Millisecond
	ccfg.ServerHealthInterval = 10 * time.Millisecond
	return cfg
}

// TestACLConfig returns a default configuration for testing an agent
// with ACLs.
func TestACLConfig() *Config {
	cfg := TestConfig()
	cfg.ACLDatacenter = cfg.Datacenter
	cfg.ACLDefaultPolicy = "deny"
	cfg.ACLMasterToken = "root"
	cfg.ACLAgentToken = "root"
	cfg.ACLAgentMasterToken = "towel"
	cfg.ACLEnforceVersion8 = &BoolTrue
	return cfg
}
