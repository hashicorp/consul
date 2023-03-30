package agent

import (
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"text/template"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/go-hclog"
	uuid "github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/tlsutil"
)

// TestAgent encapsulates an Agent with a default configuration and
// startup procedure suitable for testing. It panics if there are errors
// during creation or startup instead of returning errors. It manages a
// temporary data directory which is removed after shutdown.
type TestAgent struct {
	// Name is an optional name of the agent.
	Name string

	configFiles []string
	HCL         string

	// Config is the agent configuration. If Config is nil then
	// TestConfig() is used. If Config.DataDir is set then it is
	// the callers responsibility to clean up the data directory.
	// Otherwise, a temporary data directory is created and removed
	// when Shutdown() is called.
	Config *config.RuntimeConfig

	// LogOutput is the sink for the logs. If nil, logs are written to os.Stderr.
	// The io.Writer must allow concurrent reads and writes. Note that
	// bytes.Buffer is not safe for concurrent reads and writes.
	LogOutput io.Writer
	LogLevel  hclog.Level

	// DataDir may be set to a directory which exists. If is it not set,
	// TestAgent.Start will create one and set DataDir to the directory path.
	// In all cases the agent will be configured to use this path as the data directory,
	// and the directory will be removed once the test ends.
	DataDir string

	// UseHTTPS, if true, will disable the HTTP port and enable the HTTPS
	// one.
	UseHTTPS bool

	// UseGRPCTLS, if true, will disable the GRPC port and enable the GRPC+TLS
	// one.
	UseGRPCTLS bool

	// dns is a reference to the first started DNS endpoint.
	// It is valid after Start().
	dns *DNSServer

	// srv is an HTTPHandlers that may be used to test http endpoints.
	srv *HTTPHandlers

	// overrides is an hcl config source to use to override otherwise
	// non-user settable configurations
	Overrides string

	// allows the BaseDeps to be modified before starting the embedded agent
	OverrideDeps func(deps *BaseDeps)

	// Agent is the embedded consul agent.
	// It is valid after Start().
	*Agent
}

// NewTestAgent returns a started agent with the given configuration. It fails
// the test if the Agent could not be started.
func NewTestAgent(t *testing.T, hcl string) *TestAgent {
	a := StartTestAgent(t, TestAgent{HCL: hcl})
	t.Cleanup(func() { a.Shutdown() })
	return a
}

// NewTestAgent returns a started agent with the given configuration. It fails
// the test if the Agent could not be started.
// The caller is responsible for calling Shutdown() to stop the agent and remove
// temporary directories.
func NewTestAgentWithConfigFile(t *testing.T, hcl string, configFiles []string) *TestAgent {
	a := StartTestAgent(t, TestAgent{configFiles: configFiles, HCL: hcl})
	t.Cleanup(func() { a.Shutdown() })
	return a
}

// StartTestAgent and wait for it to become available. If the agent fails to
// start the test will be marked failed and execution will stop.
//
// The caller is responsible for calling Shutdown() to stop the agent and remove
// temporary directories.
func StartTestAgent(t *testing.T, a TestAgent) *TestAgent {
	t.Helper()
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		t.Helper()
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
		}
		peering {
			enabled = true
		}`, nodeID, connect.TestClusterID,
	)
}

// Start starts a test agent. It returns an error if the agent could not be started.
// If no error is returned, the caller must call Shutdown() when finished.
func (a *TestAgent) Start(t *testing.T) error {
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

	if a.LogLevel == 0 {
		a.LogLevel = testutil.TestLogLevel
	}

	logger := hclog.NewInterceptLogger(&hclog.LoggerOptions{
		Level:      a.LogLevel,
		Output:     logOutput,
		TimeFormat: "04:05.000",
		Name:       name,
	})

	portsConfig := randomPortsSource(t, a.UseHTTPS)

	// Create NodeID outside the closure, so that it does not change
	testHCLConfig := TestConfigHCL(NodeID())
	loader := func(source config.Source) (config.LoadResult, error) {
		opts := config.LoadOpts{
			DefaultConfig: source,
			HCL:           []string{testHCLConfig, portsConfig, a.HCL, hclDataDir},
			Overrides: []config.Source{
				config.FileSource{
					Name:   "test-overrides",
					Format: "hcl",
					Data:   a.Overrides},
				config.DefaultConsulSource(),
				config.DevConsulSource(),
			},
			ConfigFiles: a.configFiles,
		}
		result, err := config.Load(opts)
		if result.RuntimeConfig != nil {
			// If prom metrics need to be enabled, do not disable telemetry
			if result.RuntimeConfig.Telemetry.PrometheusOpts.Expiration > 0 {
				result.RuntimeConfig.Telemetry.Disable = false
			} else {
				result.RuntimeConfig.Telemetry.Disable = true
			}

			// Lower the resync interval for tests.
			result.RuntimeConfig.LocalProxyConfigResyncInterval = 250 * time.Millisecond
		}
		return result, err
	}
	bd, err := NewBaseDeps(loader, logOutput, logger)
	if err != nil {
		return fmt.Errorf("failed to create base deps: %w", err)
	}

	bd.Logger = logger
	// if we are not testing telemetry things, let's use a "mock" sink for metrics
	if bd.RuntimeConfig.Telemetry.Disable {
		bd.MetricsConfig = &lib.MetricsConfig{
			Handler: metrics.NewInmemSink(1*time.Second, time.Minute),
		}
	}

	if a.Config != nil && bd.RuntimeConfig.AutoReloadConfigCoalesceInterval == 0 {
		bd.RuntimeConfig.AutoReloadConfigCoalesceInterval = a.Config.AutoReloadConfigCoalesceInterval
	}
	a.Config = bd.RuntimeConfig

	if a.OverrideDeps != nil {
		a.OverrideDeps(&bd)
	}

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

	a.srv = a.Agent.httpHandlers

	if err := a.waitForUp(); err != nil {
		a.Shutdown()
		a.Agent = nil
		return fmt.Errorf("error waiting for test agent to start: %w", err)
	}

	a.dns = a.dnsServers[0]
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
		if len(a.apiServers.servers) == 0 {
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
			if err := a.RPC(context.Background(), "Catalog.ListNodes", args, &out); err != nil {
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
			_, err := a.srv.AgentSelf(resp, req)
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
	addr, err := firstAddr(a.Agent.apiServers, "http")
	if err != nil {
		// TODO: t.Fatal instead of panic
		panic("no http server registered")
	}
	return addr.String()
}

// firstAddr is used by tests to look up the address for the first server which
// matches the protocol
func firstAddr(s *apiServers, protocol string) (net.Addr, error) {
	for _, srv := range s.servers {
		if srv.Protocol == protocol {
			return srv.Addr, nil
		}
	}
	return nil, fmt.Errorf("no server registered with protocol %v", protocol)
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
		a.config.DNSDisableCompression = b
		srv.ReloadConfig(a.config)
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
func randomPortsSource(t *testing.T, useHTTPS bool) string {
	ports := freeport.GetN(t, 8)

	var http, https int
	if useHTTPS {
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
			grpc_tls = ` + strconv.Itoa(ports[7]) + `
		}
	`
}

func NodeID() string {
	id, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}
	return id
}

// TestConfig returns a unique default configuration for testing an agent.
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

	opts := config.LoadOpts{
		DefaultConfig: testsrc,
		Overrides:     sources,
	}
	r, err := config.Load(opts)
	if err != nil {
		panic("config.Load failed: " + err.Error())
	}
	for _, w := range r.Warnings {
		logger.Warn(w)
	}

	cfg := r.RuntimeConfig
	// Effectively disables the delay after root rotation before requesting CSRs
	// to make test deterministic. 0 results in default jitter being applied but a
	// tiny delay is effectively thre same.
	cfg.ConnectTestCALeafRootChangeSpread = 1 * time.Nanosecond

	// allows registering objects with the PeerName
	cfg.PeeringTestAllowPeerRegistrations = true

	return cfg
}

// TestACLConfig returns a default configuration for testing an agent
// with ACLs.
func TestACLConfig() string {
	return `
		primary_datacenter = "dc1"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				agent = "root"
				agent_recovery = "towel"
			}
		}
	`
}

const (
	TestDefaultInitialManagementToken = "d9f05e83-a7ae-47ce-839e-c0d53a68c00a"
	TestDefaultAgentRecoveryToken     = "bca580d4-db07-4074-b766-48acc9676955'"
)

type TestACLConfigParams struct {
	PrimaryDatacenter      string
	DefaultPolicy          string
	InitialManagementToken string
	AgentToken             string
	DefaultToken           string
	AgentRecoveryToken     string
	ReplicationToken       string
	EnableTokenReplication bool
}

func DefaultTestACLConfigParams() *TestACLConfigParams {
	return &TestACLConfigParams{
		PrimaryDatacenter:      "dc1",
		DefaultPolicy:          "deny",
		InitialManagementToken: TestDefaultInitialManagementToken,
		AgentToken:             TestDefaultInitialManagementToken,
		AgentRecoveryToken:     TestDefaultAgentRecoveryToken,
	}
}

func (p *TestACLConfigParams) HasConfiguredTokens() bool {
	return p.InitialManagementToken != "" ||
		p.AgentToken != "" ||
		p.DefaultToken != "" ||
		p.AgentRecoveryToken != "" ||
		p.ReplicationToken != ""
}

func TestACLConfigNew() string {
	return TestACLConfigWithParams(&TestACLConfigParams{
		PrimaryDatacenter:      "dc1",
		DefaultPolicy:          "deny",
		InitialManagementToken: "root",
		AgentToken:             "root",
		AgentRecoveryToken:     "towel",
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
			{{- if ne .InitialManagementToken ""}}
			initial_management = "{{ .InitialManagementToken }}"
			{{- end}}
			{{- if ne .AgentToken ""}}
			agent = "{{ .AgentToken }}"
			{{- end}}
			{{- if ne .AgentRecoveryToken "" }}
			agent_recovery = "{{ .AgentRecoveryToken }}"
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
		cfg = DefaultTestACLConfigParams()
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
	signer, _, err := tlsutil.GeneratePrivateKey()
	if err != nil {
		return "", "", "", err
	}

	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{Signer: signer})
	if err != nil {
		return "", "", "", err
	}

	cert, privateKey, err := tlsutil.GenerateCert(tlsutil.CertOpts{
		Signer:      signer,
		CA:          ca,
		Name:        "Test Cert Name",
		Days:        365,
		DNSNames:    []string{serverName},
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	})
	if err != nil {
		return "", "", "", err
	}

	return cert, privateKey, ca, nil
}
