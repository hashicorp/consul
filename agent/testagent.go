package agent

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
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

	// NoInitialSync determines whether an anti-entropy run
	// will be scheduled after the agent started.
	NoInitialSync bool

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
	if a.Config.DNSRecursor != "" {
		a.Config.DNSRecursors = append(a.Config.DNSRecursors, a.Config.DNSRecursor)
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
	id := UniqueID()

	for i := 10; i >= 0; i-- {
		pickRandomPorts(a.Config)

		// write the keyring
		if a.Key != "" {
			writeKey := func(key, filename string) {
				path := filepath.Join(a.Config.DataDir, filename)
				if err := initKeyring(path, key); err != nil {
					panic(fmt.Sprintf("Error creating keyring %s: %s", path, err))
				}
			}
			writeKey(a.Key, SerfLANKeyring)
			writeKey(a.Key, SerfWANKeyring)
		}

		agent, err := New(a.Config)
		if err != nil {
			panic(fmt.Sprintf("Error creating agent: %s", err))
		}

		logOutput := a.LogOutput
		if logOutput == nil {
			logOutput = os.Stderr
		}
		agent.LogOutput = logOutput
		agent.LogWriter = a.LogWriter
		agent.logger = log.New(logOutput, a.Name+" - ", log.LstdFlags)

		// we need the err var in the next exit condition
		if err := agent.Start(); err == nil {
			a.Agent = agent
			break
		} else if i == 0 {
			fmt.Println(id, a.Name, "Error starting agent:", err)
			runtime.Goexit()
		} else {
			agent.ShutdownAgent()
			agent.ShutdownEndpoints()
			wait := time.Duration(rand.Int31n(2000)) * time.Millisecond
			fmt.Println(id, a.Name, "retrying in", wait)
			time.Sleep(wait)
		}

		// Clean out the data dir if we are responsible for it before we
		// try again, since the old ports may have gotten written to
		// the data dir, such as in the Raft configuration.
		if a.DataDir != "" {
			if err := os.RemoveAll(a.DataDir); err != nil {
				fmt.Println(id, a.Name, "Error resetting data dir:", err)
				runtime.Goexit()
			}
		}
	}
	if !a.NoInitialSync {
		a.Agent.StartSync()
	}

	var out structs.IndexedNodes
	retry.Run(&panicFailer{}, func(r *retry.R) {
		if len(a.httpServers) == 0 {
			r.Fatal(a.Name, "waiting for server")
		}
		if a.Config.Bootstrap && a.Config.Server {
			// Ensure we have a leader and a node registration.
			args := &structs.DCSpecificRequest{
				Datacenter: a.Config.Datacenter,
				QueryOptions: structs.QueryOptions{
					MinQueryIndex: out.Index,
					MaxQueryTime:  25 * time.Millisecond,
				},
			}
			if err := a.RPC("Catalog.ListNodes", args, &out); err != nil {
				r.Fatal(a.Name, "Catalog.ListNodes failed:", err)
			}
			if !out.QueryMeta.KnownLeader {
				r.Fatal(a.Name, "No leader")
			}
			if out.Index == 0 {
				r.Fatal(a.Name, "Consul index is 0")
			}
		} else {
			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.httpServers[0].AgentSelf(resp, req)
			if err != nil || resp.Code != 200 {
				r.Fatal(a.Name, "failed OK respose", err)
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

	// shutdown agent before endpoints
	defer a.Agent.ShutdownEndpoints()
	return a.Agent.ShutdownAgent()
}

func (a *TestAgent) HTTPAddr() string {
	if a.srv == nil {
		return ""
	}
	return a.srv.Addr
}

func (a *TestAgent) Client() *api.Client {
	conf := api.DefaultConfig()
	conf.Address = a.HTTPAddr()
	c, err := api.NewClient(conf)
	if err != nil {
		panic(fmt.Sprintf("Error creating consul API client: %s", err))
	}
	return c
}

func (a *TestAgent) consulConfig() *consul.Config {
	c, err := a.Agent.consulConfig()
	if err != nil {
		panic(err)
	}
	return c
}

func UniqueID() string {
	id := strconv.FormatUint(rand.Uint64(), 36)
	for len(id) < 16 {
		id += " "
	}
	return id
}

// TenPorts returns the first port number of a block of
// ten random ports.
func TenPorts() int {
	return 1030 + int(rand.Int31n(6440))*10
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
	port := TenPorts()
	c.Ports.DNS = port + 1
	c.Ports.HTTP = port + 2
	// when we enable HTTPS then we need to fix finding the
	// "first" HTTP server since that might be HTTPS server
	// c.Ports.HTTPS = port + 3
	c.Ports.SerfLan = port + 4
	c.Ports.SerfWan = port + 5
	c.Ports.Server = port + 6
}

// TestConfig returns a unique default configuration for testing an
// agent.
func TestConfig() *Config {
	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	cfg := DefaultConfig()

	cfg.Version = version.Version
	cfg.VersionPrerelease = "c.d"

	cfg.NodeID = types.NodeID(nodeID)
	cfg.NodeName = "Node " + nodeID
	cfg.BindAddr = "127.0.0.1"
	cfg.AdvertiseAddr = "127.0.0.1"
	cfg.Datacenter = "dc1"
	cfg.Bootstrap = true
	cfg.Server = true

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
	cfg.ACLEnforceVersion8 = Bool(true)
	return cfg
}
