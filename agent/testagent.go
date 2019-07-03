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
	"strconv"
	"strings"
	"testing"
	"time"

	metrics "github.com/armon/go-metrics"
	uuid "github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"

	"github.com/stretchr/testify/require"
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

	HCL string

	// ExpectConfigError can be set to prevent the agent retrying Start on errors
	// and eventually blowing up with runtime.Goexit. This enables tests to assert
	// that some specific bit of config actually does prevent startup entirely in
	// a reasonable way without reproducing a lot of the boilerplate here.
	ExpectConfigError bool

	// Config is the agent configuration. If Config is nil then
	// TestConfig() is used. If Config.DataDir is set then it is
	// the callers responsibility to clean up the data directory.
	// Otherwise, a temporary data directory is created and removed
	// when Shutdown() is called.
	Config *config.RuntimeConfig

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

	// UseTLS, if true, will disable the HTTP port and enable the HTTPS
	// one.
	UseTLS bool

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
// configuration. It fails the test if the Agent could not be started. The
// caller should call Shutdown() to stop the agent and remove temporary
// directories.
func NewTestAgent(t *testing.T, name string, hcl string) *TestAgent {
	a := &TestAgent{Name: name, HCL: hcl}
	a.Start(t)
	return a
}

func NewUnstartedAgent(t *testing.T, name string, hcl string) (*Agent, error) {
	c := TestConfig(config.Source{Name: name, Format: "hcl", Data: hcl})
	a, err := New(c, nil)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// Start starts a test agent. It fails the test if the agent could not be started.
func (a *TestAgent) Start(t *testing.T) *TestAgent {
	require := require.New(t)
	require.Nil(a.Agent, "TestAgent already started")
	var hclDataDir string
	if a.DataDir == "" {
		name := "agent"
		if a.Name != "" {
			name = a.Name + "-agent"
		}
		name = strings.Replace(name, "/", "_", -1)
		d, err := ioutil.TempDir(TempDir, name)
		require.NoError(err, fmt.Sprintf("Error creating data dir %s: %s", filepath.Join(TempDir, name), err))
		hclDataDir = `data_dir = "` + d + `"`
	}

	var id string
	for i := 10; i >= 0; i-- {
		a.Config = TestConfig(
			randomPortsSource(a.UseTLS),
			config.Source{Name: a.Name, Format: "hcl", Data: a.HCL},
			config.Source{Name: a.Name + ".data_dir", Format: "hcl", Data: hclDataDir},
		)
		id = string(a.Config.NodeID)

		// write the keyring
		if a.Key != "" {
			writeKey := func(key, filename string) {
				path := filepath.Join(a.Config.DataDir, filename)
				err := initKeyring(path, key)
				require.NoError(err, fmt.Sprintf("Error creating keyring %s: %s", path, err))
			}
			writeKey(a.Key, SerfLANKeyring)
			writeKey(a.Key, SerfWANKeyring)
		}

		logOutput := a.LogOutput
		if logOutput == nil {
			logOutput = os.Stderr
		}
		agentLogger := log.New(logOutput, a.Name+" - ", log.LstdFlags|log.Lmicroseconds)

		agent, err := New(a.Config, agentLogger)
		require.NoError(err, fmt.Sprintf("Error creating agent: %s", err))

		agent.LogOutput = logOutput
		agent.LogWriter = a.LogWriter
		agent.MemSink = metrics.NewInmemSink(1*time.Second, time.Minute)

		// we need the err var in the next exit condition
		if err := agent.Start(); err == nil {
			a.Agent = agent
			break
		} else if i == 0 {
			require.Fail("%s %s Error starting agent: %s", id, a.Name, err)
		} else if a.ExpectConfigError {
			// Panic the error since this can be caught if needed. Pretty gross way to
			// detect errors but enough for now and this is a tiny edge case that I'd
			// otherwise not have a way to test at all...
			panic(err)
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
				require.Fail("%s %s Error resetting data dir: %s", id, a.Name, err)
			}
		}
	}

	// Start the anti-entropy syncer
	a.Agent.StartSync()

	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		if len(a.httpServers) == 0 {
			r.Fatal(a.Name, "waiting for server")
		}
		if a.Config.Bootstrap && a.Config.ServerMode {
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
				r.Fatal(a.Name, ": Consul index is 0")
			}
		} else {
			req, _ := http.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.httpServers[0].AgentSelf(resp, req)
			if err != nil || resp.Code != 200 {
				r.Fatal(a.Name, "failed OK response", err)
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
	/* Removed this because it was breaking persistence tests where we would
	persist a service and load it through a new agent with the same data-dir.
	Not sure if we still need this for other things, everywhere we manually make
	a data dir we already do 'defer os.RemoveAll()'
	defer func() {
		if a.DataDir != "" {
			os.RemoveAll(a.DataDir)
		}
	}()*/

	// shutdown agent before endpoints
	defer a.Agent.ShutdownEndpoints()
	return a.Agent.ShutdownAgent()
}

func (a *TestAgent) DNSAddr() string {
	if a.dns == nil {
		return ""
	}
	return a.dns.Addr
}

func (a *TestAgent) HTTPAddr() string {
	if a.srv == nil {
		return ""
	}
	return a.srv.Addr
}

func (a *TestAgent) SegmentAddr(name string) string {
	if server, ok := a.Agent.delegate.(*consul.Server); ok {
		return server.LANSegmentAddr(name)
	}
	return ""
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

// DNSDisableCompression disables compression for all started DNS servers.
func (a *TestAgent) DNSDisableCompression(b bool) {
	for _, srv := range a.dnsServers {
		cfg := srv.config.Load().(*dnsConfig)
		cfg.DisableCompression = b
	}
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
// reduces it significantly with little overhead. Furthermore, asking
// the kernel for a random port by binding to port 0 prolongs the test
// execution (in our case +20sec) while also not fully eliminating the
// chance of port conflicts for concurrently executed test binaries.
// Instead of relying on one set of ports to be sufficient we retry
// starting the agent with different ports on port conflict.
func randomPortsSource(tls bool) config.Source {
	ports := freeport.Get(6)
	if tls {
		ports[1] = -1
	} else {
		ports[2] = -1
	}
	return config.Source{
		Name:   "ports",
		Format: "hcl",
		Data: `
			ports = {
				dns = ` + strconv.Itoa(ports[0]) + `
				http = ` + strconv.Itoa(ports[1]) + `
				https = ` + strconv.Itoa(ports[2]) + `
				serf_lan = ` + strconv.Itoa(ports[3]) + `
				serf_wan = ` + strconv.Itoa(ports[4]) + `
				server = ` + strconv.Itoa(ports[5]) + `
			}
		`,
	}
}

func NodeID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id
}

// TestConfig returns a unique default configuration for testing an
// agent.
func TestConfig(sources ...config.Source) *config.RuntimeConfig {
	nodeID := NodeID()
	testsrc := config.Source{
		Name:   "test",
		Format: "hcl",
		Data: `
			bind_addr = "127.0.0.1"
			advertise_addr = "127.0.0.1"
			datacenter = "dc1"
			bootstrap = true
			server = true
			node_id = "` + nodeID + `"
			node_name = "Node ` + nodeID + `"
			connect {
				enabled = true
				ca_config {
					cluster_id = "` + connect.TestClusterID + `"
				}
			}
			performance {
				raft_multiplier = 1
			}
		`,
	}

	b, err := config.NewBuilder(config.Flags{})
	if err != nil {
		panic("NewBuilder failed: " + err.Error())
	}
	b.Head = append(b.Head, testsrc)
	b.Tail = append(b.Tail, config.DefaultConsulSource(), config.DevConsulSource())
	b.Tail = append(b.Tail, sources...)

	cfg, err := b.BuildAndValidate()
	if err != nil {
		panic("Error building config: " + err.Error())
	}

	for _, w := range b.Warnings {
		fmt.Println("WARNING:", w)
	}

	// Disable connect proxy execution since it causes all kinds of problems with
	// self-executing tests etc.
	cfg.ConnectTestDisableManagedProxies = true
	// Effectively disables the delay after root rotation before requesting CSRs
	// to make test deterministic. 0 results in default jitter being applied but a
	// tiny delay is effectively thre same.
	cfg.ConnectTestCALeafRootChangeSpread = 1 * time.Nanosecond

	return &cfg
}

// TestACLConfig returns a default configuration for testing an agent
// with ACLs.
func TestACLConfig() string {
	return `
		acl_datacenter = "dc1"
		acl_default_policy = "deny"
		acl_master_token = "root"
		acl_agent_token = "root"
		acl_agent_master_token = "towel"
		acl_enforce_version_8 = true
	`
}

func TestACLConfigNew() string {
	return `
		primary_datacenter = "dc1"
		acl {
			enabled = true
			default_policy = "deny"
			tokens {
				master = "root"
				agent = "root"
				agent_master = "towel"
			}
		}
	`
}
