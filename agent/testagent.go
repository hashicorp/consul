package agent

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"fmt"
	"io"
	"math/rand"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"text/template"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/errwrap"
	"github.com/hashicorp/go-hclog"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/tlsutil"
)

func init() {
	rand.Seed(time.Now().UnixNano()) // seed random number generator
}

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

	// LogOutput is the sink for the logs. If nil, logs are written
	// to os.Stderr.
	LogOutput io.Writer

	// DataDir may be set to a directory which exists. If is it not set,
	// TestAgent.Start will create one and set DataDir to the directory path.
	// In all cases the agent will be configured to use this path as the data directory,
	// and the directory will be removed once the test ends.
	DataDir string

	// UseTLS, if true, will disable the HTTP port and enable the HTTPS
	// one.
	UseTLS bool

	// dns is a reference to the first started DNS endpoint.
	// It is valid after Start().
	dns *DNSServer

	// srv is a reference to the first started HTTP endpoint.
	// It is valid after Start().
	srv *HTTPServer

	// overrides is an hcl config source to use to override otherwise
	// non-user settable configurations
	Overrides string

	// Agent is the embedded consul agent.
	// It is valid after Start().
	*Agent
}

// NewTestAgent returns a started agent with the given configuration. It fails
// the test if the Agent could not be started.
// The caller is responsible for calling Shutdown() to stop the agent and remove
// temporary directories.
func NewTestAgent(t *testing.T, hcl string) *TestAgent {
	return StartTestAgent(t, TestAgent{HCL: hcl})
}

// StartTestAgent and wait for it to become available. If the agent fails to
// start the test will be marked failed and execution will stop.
//
// The caller is responsible for calling Shutdown() to stop the agent and remove
// temporary directories.
func StartTestAgent(t *testing.T, a TestAgent) *TestAgent {
	t.Helper()
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		if err := a.Start(t); err != nil {
			r.Fatal(err)
		}
	})

	return &a
}

func TestConfigHCL(nodeID string) string {
	return fmt.Sprintf(`
		bind_addr = "127.0.0.1"
		advertise_addr = "127.0.0.1"
		datacenter = "dc1"
		bootstrap = true
		server = true
		node_id = "%[1]s"
		node_name = "Node-%[1]s"
		connect {
			enabled = true
			ca_config {
				cluster_id = "%[2]s"
			}
		}
		performance {
			raft_multiplier = 1
		}`, nodeID, connect.TestClusterID,
	)
}

// Start starts a test agent. It returns an error if the agent could not be started.
// If no error is returned, the caller must call Shutdown() when finished.
func (a *TestAgent) Start(t *testing.T) (err error) {
	t.Helper()
	if a.Agent != nil {
		return fmt.Errorf("TestAgent already started")
	}

	name := a.Name
	if name == "" {
		name = "TestAgent"
	}

	if a.DataDir == "" {
		dirname := name + "-agent"
		a.DataDir = testutil.TempDir(t, dirname)
	}
	// Convert windows style path to posix style path to avoid illegal char escape
	// error when hcl parsing.
	d := filepath.ToSlash(a.DataDir)
	hclDataDir := fmt.Sprintf(`data_dir = "%s"`, d)

	logOutput := a.LogOutput
	if logOutput == nil {
		logOutput = testutil.NewLogBuffer(t)
	}

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Level:      hclog.Debug,
		Output:     logOutput,
		TimeFormat: "04:05.000",
		Name:       name,
	})

	portsConfig, returnPortsFn := randomPortsSource(a.UseTLS)
	t.Cleanup(returnPortsFn)

	// Create NodeID outside the closure, so that it does not change
	testHCLConfig := TestConfigHCL(NodeID())
	loader := func(source config.Source) (*config.RuntimeConfig, []string, error) {
		opts := config.BuilderOpts{
			HCL: []string{testHCLConfig, portsConfig, a.HCL, hclDataDir},
		}
		overrides := []config.Source{
			config.FileSource{
				Name:   "test-overrides",
				Format: "hcl",
				Data:   a.Overrides},
			config.DefaultConsulSource(),
			config.DevConsulSource(),
		}
		cfg, warnings, err := config.Load(opts, source, overrides...)
		if cfg != nil {
			cfg.Telemetry.Disable = true
		}
		return cfg, warnings, err
	}
	bd, err := NewBaseDeps(loader, logOutput)
	require.NoError(t, err)

	bd.Logger = logger
	bd.MetricsHandler = metrics.NewInmemSink(1*time.Second, time.Minute)
	a.Config = bd.RuntimeConfig

	agent, err := New(bd)
	if err != nil {
		return fmt.Errorf("Error creating agent: %s", err)
	}

	id := string(a.Config.NodeID)
	if err := agent.Start(context.Background()); err != nil {
		agent.ShutdownAgent()
		agent.ShutdownEndpoints()

		return fmt.Errorf("%s %s Error starting agent: %s", id, name, err)
	}

	a.Agent = agent

	// Start the anti-entropy syncer
	a.Agent.StartSync()

	if err := a.waitForUp(); err != nil {
		a.Shutdown()
		t.Logf("Error while waiting for test agent to start: %v", err)
		return errwrap.Wrapf(name+": {{err}}", err)
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
			retErr = fmt.Errorf("waiting for server")
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
				retErr = fmt.Errorf("No leader")
				continue // fail, try again
			}
			if out.Index == 0 {
				retErr = fmt.Errorf("Consul index is 0")
				continue // fail, try again
			}
			return nil // success
		} else {
			req := httptest.NewRequest("GET", "/v1/agent/self", nil)
			resp := httptest.NewRecorder()
			_, err := a.httpServers[0].AgentSelf(resp, req)
			if acl.IsErrPermissionDenied(err) || resp.Code == 403 {
				// permission denied is enough to show that the client is
				// connected to the servers as it would get a 503 if
				// it couldn't connect to them.
			} else if err != nil && resp.Code != 200 {
				retErr = fmt.Errorf("failed OK response: %v", err)
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
	return a.srv.Server.Addr
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

// FIXME: this should t.Fatal on error, not panic.
// TODO: rename to newConsulConfig
// TODO: remove TestAgent receiver, accept a.Agent.config as an arg
func (a *TestAgent) consulConfig() *consul.Config {
	c, err := newConsulConfig(a.Agent.config, a.Agent.logger)
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
func randomPortsSource(tls bool) (data string, returnPortsFn func()) {
	ports := freeport.MustTake(7)

	var http, https int
	if tls {
		http = -1
		https = ports[2]
	} else {
		http = ports[1]
		https = -1
	}

	return `
		ports = {
			dns = ` + strconv.Itoa(ports[0]) + `
			http = ` + strconv.Itoa(http) + `
			https = ` + strconv.Itoa(https) + `
			serf_lan = ` + strconv.Itoa(ports[3]) + `
			serf_wan = ` + strconv.Itoa(ports[4]) + `
			server = ` + strconv.Itoa(ports[5]) + `
			grpc = ` + strconv.Itoa(ports[6]) + `
		}
	`, func() { freeport.Return(ports) }
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
	testsrc := config.FileSource{
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

	b, err := config.NewBuilder(config.BuilderOpts{})
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
   {{- if ne .PrimaryDatacenter "" -}}
	primary_datacenter = "{{ .PrimaryDatacenter }}"
	{{end -}}
	acl {
		enabled = true
		{{- if ne .DefaultPolicy ""}}
		default_policy = "{{ .DefaultPolicy }}"
		{{- end}}
		enable_token_replication = {{printf "%t" .EnableTokenReplication }}
		{{- if .HasConfiguredTokens}}
		tokens {
			{{- if ne .MasterToken ""}}
			master = "{{ .MasterToken }}"
			{{- end}}
			{{- if ne .AgentToken ""}}
			agent = "{{ .AgentToken }}"
			{{- end}}
			{{- if ne .AgentMasterToken "" }}
			agent_master = "{{ .AgentMasterToken }}"
			{{- end}}
			{{- if ne .DefaultToken "" }}
			default = "{{ .DefaultToken }}"
			{{- end}}
			{{- if ne .ReplicationToken ""  }}
			replication = "{{ .ReplicationToken }}"
			{{- end}}
		}
		{{- end}}
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

// testTLSCertificates Generates a TLS CA and server key/cert and returns them
// in PEM encoded form.
func testTLSCertificates(serverName string) (cert string, key string, cacert string, err error) {
	// generate CA
	serial, err := tlsutil.GenerateSerialNumber()
	if err != nil {
		return "", "", "", err
	}
	signer, err := ecdsa.GenerateKey(elliptic.P256(), rand.New(rand.NewSource(99)))
	if err != nil {
		return "", "", "", err
	}
	ca, err := tlsutil.GenerateCA(signer, serial, 365, nil)
	if err != nil {
		return "", "", "", err
	}

	// generate leaf
	serial, err = tlsutil.GenerateSerialNumber()
	if err != nil {
		return "", "", "", err
	}

	cert, privateKey, err := tlsutil.GenerateCert(
		signer,
		ca,
		serial,
		"Test Cert Name",
		365,
		[]string{serverName},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	)
	if err != nil {
		return "", "", "", err
	}

	return cert, privateKey, ca, nil
}
