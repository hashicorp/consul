package agent

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	uuid "github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
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

	// Config is the agent configuration. If Config is nil then
	// TestConfig() is used. If Config.DataDir is set then it is
	// the callers responsibility to clean up the data directory.
	// Otherwise, a temporary data directory is created and removed
	// when Shutdown() is called.
	Config *config.RuntimeConfig

	// returnPortsFn will put the ports claimed for the test back into the
	// general freeport pool
	returnPortsFn func()

	// LogOutput is the sink for the logs. If nil, logs are written
	// to os.Stderr.
	LogOutput io.Writer

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
	return NewTestAgentWithFields(t, true, TestAgent{Name: name, HCL: hcl})
}

// NewTestAgentWithFields takes a TestAgent struct with any number of fields set,
// and a boolean 'start', which indicates whether or not the TestAgent should
// be started. If no LogOutput is set, it will automatically be set to
// testutil.TestWriter(t). Name will default to t.Name() if not specified.
func NewTestAgentWithFields(t *testing.T, start bool, ta TestAgent) *TestAgent {
	// copy values
	a := ta
	if a.Name == "" {
		a.Name = t.Name()
	}
	if a.LogOutput == nil {
		a.LogOutput = testutil.TestWriter(t)
	}
	if !start {
		return nil
	}

	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		if err := a.Start(); err != nil {
			r.Fatal(err)
		}
	})

	return &a
}

// Start starts a test agent. It returns an error if the agent could not be started.
// If no error is returned, the caller must call Shutdown() when finished.
func (a *TestAgent) Start() (err error) {
	if a.Agent != nil {
		return fmt.Errorf("TestAgent already started")
	}

	var cleanupTmpDir = func() {
		// Clean out the data dir if we are responsible for it before we
		// try again, since the old ports may have gotten written to
		// the data dir, such as in the Raft configuration.
		if a.DataDir != "" {
			if err := os.RemoveAll(a.DataDir); err != nil {
				fmt.Printf("%s Error resetting data dir: %s", a.Name, err)
			}
		}
	}

	var hclDataDir string
	if a.DataDir == "" {
		name := "agent"
		if a.Name != "" {
			name = a.Name + "-agent"
		}
		name = strings.Replace(name, "/", "_", -1)
		d, err := ioutil.TempDir(TempDir, name)
		if err != nil {
			return fmt.Errorf("Error creating data dir %s: %s", filepath.Join(TempDir, name), err)
		}
		// Convert windows style path to posix style path
		// to avoid illegal char escape error when hcl
		// parsing.
		d = filepath.ToSlash(d)
		hclDataDir = `data_dir = "` + d + `"`
	}

	logOutput := a.LogOutput
	if logOutput == nil {
		logOutput = os.Stderr
	}

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Name:   a.Name,
		Level:  hclog.Debug,
		Output: logOutput,
	})

	portsConfig, returnPortsFn := randomPortsSource(a.UseTLS)
	a.returnPortsFn = returnPortsFn
	a.Config = TestConfig(logger,
		portsConfig,
		config.Source{Name: a.Name, Format: "hcl", Data: a.HCL},
		config.Source{Name: a.Name + ".data_dir", Format: "hcl", Data: hclDataDir},
	)

	defer func() {
		if err != nil && a.returnPortsFn != nil {
			a.returnPortsFn()
			a.returnPortsFn = nil
		}
	}()

	// write the keyring
	if a.Key != "" {
		writeKey := func(key, filename string) error {
			path := filepath.Join(a.Config.DataDir, filename)
			if err := initKeyring(path, key); err != nil {
				cleanupTmpDir()
				return fmt.Errorf("Error creating keyring %s: %s", path, err)
			}
			return nil
		}
		if err = writeKey(a.Key, SerfLANKeyring); err != nil {
			cleanupTmpDir()
			return err
		}
		if err = writeKey(a.Key, SerfWANKeyring); err != nil {
			cleanupTmpDir()
			return err
		}
	}

	agent, err := New(a.Config, logger)
	if err != nil {
		cleanupTmpDir()
		return fmt.Errorf("Error creating agent: %s", err)
	}

	agent.LogOutput = logOutput
	agent.MemSink = metrics.NewInmemSink(1*time.Second, time.Minute)

	id := string(a.Config.NodeID)

	if err := agent.Start(); err != nil {
		cleanupTmpDir()
		agent.ShutdownAgent()
		agent.ShutdownEndpoints()

		return fmt.Errorf("%s %s Error starting agent: %s", id, a.Name, err)
	}

	a.Agent = agent

	// Start the anti-entropy syncer
	a.Agent.StartSync()

	if err := a.waitForUp(); err != nil {
		cleanupTmpDir()
		a.Shutdown()
		return err
	}

	a.dns = a.dnsServers[0]
	a.srv = a.httpServers[0]
	return nil
}

// waitForUp waits for leader election, or waits for the agent HTTP
// endpoint to start responding, depending on the agent config.
func (a *TestAgent) waitForUp() error {
	timer := retry.TwoSeconds()
	deadline := time.Now().Add(timer.Timeout)

	var retErr error
	var out structs.IndexedNodes
	for ; !time.Now().After(deadline); time.Sleep(timer.Wait) {
		if len(a.httpServers) == 0 {
			retErr = fmt.Errorf("%s: waiting for server", a.Name)
			continue // fail, try again
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
				retErr = fmt.Errorf("Catalog.ListNodes failed: %v", err)
				continue // fail, try again
			}
			if !out.QueryMeta.KnownLeader {
				retErr = fmt.Errorf("%s: No leader", a.Name)
				continue // fail, try again
			}
			if out.Index == 0 {
				retErr = fmt.Errorf("%s: Consul index is 0", a.Name)
				continue // fail, try again
			}
			return nil // success
		} else {
			req := httptest.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.httpServers[0].AgentSelf(resp, req)
			if err != nil || resp.Code != 200 {
				retErr = fmt.Errorf("%s: failed OK response: %v", a.Name, err)
				continue
			}
			return nil // success
		}
	}

	return fmt.Errorf("unavailable. last error: %v", retErr)
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

	// already shut down
	if a.Agent == nil {
		return nil
	}

	// Return ports last of all
	defer func() {
		if a.returnPortsFn != nil {
			a.returnPortsFn()
			a.returnPortsFn = nil
		}
	}()

	// shutdown agent before endpoints
	defer a.Agent.ShutdownEndpoints()
	if err := a.Agent.ShutdownAgent(); err != nil {
		return err
	}
	<-a.Agent.ShutdownCh()
	return nil
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
func randomPortsSource(tls bool) (src config.Source, returnPortsFn func()) {
	ports := freeport.MustTake(7)

	var http, https int
	if tls {
		http = -1
		https = ports[2]
	} else {
		http = ports[1]
		https = -1
	}

	return config.Source{
		Name:   "ports",
		Format: "hcl",
		Data: `
			ports = {
				dns = ` + strconv.Itoa(ports[0]) + `
				http = ` + strconv.Itoa(http) + `
				https = ` + strconv.Itoa(https) + `
				serf_lan = ` + strconv.Itoa(ports[3]) + `
				serf_wan = ` + strconv.Itoa(ports[4]) + `
				server = ` + strconv.Itoa(ports[5]) + `
				grpc = ` + strconv.Itoa(ports[6]) + `
			}
		`,
	}, func() { freeport.Return(ports) }
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
func TestConfig(logger hclog.Logger, sources ...config.Source) *config.RuntimeConfig {
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
			node_name = "Node-` + nodeID + `"
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
		logger.Warn(w)
	}

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

const (
	TestDefaultMasterToken      = "d9f05e83-a7ae-47ce-839e-c0d53a68c00a"
	TestDefaultAgentMasterToken = "bca580d4-db07-4074-b766-48acc9676955'"
)

type TestACLConfigParams struct {
	PrimaryDatacenter      string
	DefaultPolicy          string
	MasterToken            string
	AgentToken             string
	DefaultToken           string
	AgentMasterToken       string
	ReplicationToken       string
	EnableTokenReplication bool
}

func DefaulTestACLConfigParams() *TestACLConfigParams {
	return &TestACLConfigParams{
		PrimaryDatacenter: "dc1",
		DefaultPolicy:     "deny",
		MasterToken:       TestDefaultMasterToken,
		AgentToken:        TestDefaultMasterToken,
		AgentMasterToken:  TestDefaultAgentMasterToken,
	}
}

func (p *TestACLConfigParams) HasConfiguredTokens() bool {
	return p.MasterToken != "" ||
		p.AgentToken != "" ||
		p.DefaultToken != "" ||
		p.AgentMasterToken != "" ||
		p.ReplicationToken != ""
}

func TestACLConfigNew() string {
	return TestACLConfigWithParams(&TestACLConfigParams{
		PrimaryDatacenter: "dc1",
		DefaultPolicy:     "deny",
		MasterToken:       "root",
		AgentToken:        "root",
		AgentMasterToken:  "towel",
	})
}

var aclConfigTpl = template.Must(template.New("ACL Config").Parse(`
   {{if ne .PrimaryDatacenter ""}}
	primary_datacenter = "{{ .PrimaryDatacenter }}"
	{{end}}
	acl {
		enabled = true
		{{if ne .DefaultPolicy ""}}
		default_policy = "{{ .DefaultPolicy }}"
		{{end}}
		enable_token_replication = {{printf "%t" .EnableTokenReplication }}
		{{if .HasConfiguredTokens }}
		tokens {
			{{if ne .MasterToken ""}}
			master = "{{ .MasterToken }}"
			{{end}}
			{{if ne .AgentToken ""}}
			agent = "{{ .AgentToken }}"
			{{end}}
			{{if ne .AgentMasterToken "" }}
			agent_master = "{{ .AgentMasterToken }}"
			{{end}}
			{{if ne .DefaultToken "" }}
			default = "{{ .DefaultToken }}"
			{{end}}
			{{if ne .ReplicationToken "" }}
			replication = "{{ .ReplicationToken }}"
			{{end}}
		}
		{{end}}
	}
`))

func TestACLConfigWithParams(params *TestACLConfigParams) string {
	var buf bytes.Buffer

	cfg := params
	if params == nil {
		cfg = DefaulTestACLConfigParams()
	}

	err := aclConfigTpl.Execute(&buf, &cfg)
	if err != nil {
		panic(fmt.Sprintf("Failed to generate test ACL config: %v", err))
	}

	return buf.String()
}
