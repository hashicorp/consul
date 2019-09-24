package testutil

// TestServer is a test helper. It uses a fork/exec model to create
// a test Consul server instance in the background and initialize it
// with some data and/or services. The test server can then be used
// to run a unit test, and offers an easy API to tear itself down
// when the test has completed. The only prerequisite is to have a consul
// binary available on the $PATH.
//
// This package does not use Consul's official API client. This is
// because we use TestServer to test the API client, which would
// otherwise cause an import cycle.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-uuid"
	"github.com/pkg/errors"
)

// TestPerformanceConfig configures the performance parameters.
type TestPerformanceConfig struct {
	RaftMultiplier uint `json:"raft_multiplier,omitempty"`
}

// TestPortConfig configures the various ports used for services
// provided by the Consul server.
type TestPortConfig struct {
	DNS          int `json:"dns,omitempty"`
	HTTP         int `json:"http,omitempty"`
	HTTPS        int `json:"https,omitempty"`
	SerfLan      int `json:"serf_lan,omitempty"`
	SerfWan      int `json:"serf_wan,omitempty"`
	Server       int `json:"server,omitempty"`
	ProxyMinPort int `json:"proxy_min_port,omitempty"`
	ProxyMaxPort int `json:"proxy_max_port,omitempty"`
}

// TestAddressConfig contains the bind addresses for various
// components of the Consul server.
type TestAddressConfig struct {
	HTTP string `json:"http,omitempty"`
}

// TestNetworkSegment contains the configuration for a network segment.
type TestNetworkSegment struct {
	Name      string `json:"name"`
	Bind      string `json:"bind"`
	Port      int    `json:"port"`
	Advertise string `json:"advertise"`
}

// TestServerConfig is the main server configuration struct.
type TestServerConfig struct {
	NodeName            string                 `json:"node_name"`
	NodeID              string                 `json:"node_id"`
	NodeMeta            map[string]string      `json:"node_meta,omitempty"`
	Performance         *TestPerformanceConfig `json:"performance,omitempty"`
	Bootstrap           bool                   `json:"bootstrap,omitempty"`
	Server              bool                   `json:"server,omitempty"`
	DataDir             string                 `json:"data_dir,omitempty"`
	Datacenter          string                 `json:"datacenter,omitempty"`
	Segments            []TestNetworkSegment   `json:"segments"`
	DisableCheckpoint   bool                   `json:"disable_update_check"`
	LogLevel            string                 `json:"log_level,omitempty"`
	Bind                string                 `json:"bind_addr,omitempty"`
	Addresses           *TestAddressConfig     `json:"addresses,omitempty"`
	Ports               *TestPortConfig        `json:"ports,omitempty"`
	RaftProtocol        int                    `json:"raft_protocol,omitempty"`
	ACLMasterToken      string                 `json:"acl_master_token,omitempty"`
	ACLDatacenter       string                 `json:"acl_datacenter,omitempty"`
	PrimaryDatacenter   string                 `json:"primary_datacenter,omitempty"`
	ACLDefaultPolicy    string                 `json:"acl_default_policy,omitempty"`
	ACLEnforceVersion8  bool                   `json:"acl_enforce_version_8"`
	ACL                 TestACLs               `json:"acl,omitempty"`
	Encrypt             string                 `json:"encrypt,omitempty"`
	CAFile              string                 `json:"ca_file,omitempty"`
	CertFile            string                 `json:"cert_file,omitempty"`
	KeyFile             string                 `json:"key_file,omitempty"`
	VerifyIncoming      bool                   `json:"verify_incoming,omitempty"`
	VerifyIncomingRPC   bool                   `json:"verify_incoming_rpc,omitempty"`
	VerifyIncomingHTTPS bool                   `json:"verify_incoming_https,omitempty"`
	VerifyOutgoing      bool                   `json:"verify_outgoing,omitempty"`
	EnableScriptChecks  bool                   `json:"enable_script_checks,omitempty"`
	Connect             map[string]interface{} `json:"connect,omitempty"`
	EnableDebug         bool                   `json:"enable_debug,omitempty"`
	ReadyTimeout        time.Duration          `json:"-"`
	Stdout, Stderr      io.Writer              `json:"-"`
	Args                []string               `json:"-"`
	ReturnPorts         func()                 `json:"-"`
}

type TestACLs struct {
	Enabled             bool       `json:"enabled,omitempty"`
	TokenReplication    bool       `json:"enable_token_replication,omitempty"`
	PolicyTTL           string     `json:"policy_ttl,omitempty"`
	TokenTTL            string     `json:"token_ttl,omitempty"`
	DownPolicy          string     `json:"down_policy,omitempty"`
	DefaultPolicy       string     `json:"default_policy,omitempty"`
	EnableKeyListPolicy bool       `json:"enable_key_list_policy,omitempty"`
	Tokens              TestTokens `json:"tokens,omitempty"`
	DisabledTTL         string     `json:"disabled_ttl,omitempty"`
}

type TestTokens struct {
	Master      string `json:"master,omitempty"`
	Replication string `json:"replication,omitempty"`
	AgentMaster string `json:"agent_master,omitempty"`
	Default     string `json:"default,omitempty"`
	Agent       string `json:"agent,omitempty"`
}

// ServerConfigCallback is a function interface which can be
// passed to NewTestServerConfig to modify the server config.
type ServerConfigCallback func(c *TestServerConfig)

// defaultServerConfig returns a new TestServerConfig struct
// with all of the listen ports incremented by one.
func defaultServerConfig() *TestServerConfig {
	nodeID, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	ports := freeport.MustTake(6)

	return &TestServerConfig{
		NodeName:          "node-" + nodeID,
		NodeID:            nodeID,
		DisableCheckpoint: true,
		Performance: &TestPerformanceConfig{
			RaftMultiplier: 1,
		},
		Bootstrap: true,
		Server:    true,
		LogLevel:  "debug",
		Bind:      "127.0.0.1",
		Addresses: &TestAddressConfig{},
		Ports: &TestPortConfig{
			DNS:     ports[0],
			HTTP:    ports[1],
			HTTPS:   ports[2],
			SerfLan: ports[3],
			SerfWan: ports[4],
			Server:  ports[5],
		},
		ReadyTimeout: 10 * time.Second,
		Connect: map[string]interface{}{
			"enabled": true,
			"ca_config": map[string]interface{}{
				// const TestClusterID causes import cycle so hard code it here.
				"cluster_id": "11111111-2222-3333-4444-555555555555",
			},
		},
		ReturnPorts: func() {
			freeport.Return(ports)
		},
	}
}

// TestService is used to serialize a service definition.
type TestService struct {
	ID      string   `json:",omitempty"`
	Name    string   `json:",omitempty"`
	Tags    []string `json:",omitempty"`
	Address string   `json:",omitempty"`
	Port    int      `json:",omitempty"`
}

// TestCheck is used to serialize a check definition.
type TestCheck struct {
	ID        string `json:",omitempty"`
	Name      string `json:",omitempty"`
	ServiceID string `json:",omitempty"`
	TTL       string `json:",omitempty"`
}

// TestKVResponse is what we use to decode KV data.
type TestKVResponse struct {
	Value string
}

// TestServer is the main server wrapper struct.
type TestServer struct {
	cmd    *exec.Cmd
	Config *TestServerConfig

	HTTPAddr  string
	HTTPSAddr string
	LANAddr   string
	WANAddr   string

	HTTPClient *http.Client

	tmpdir string
}

// NewTestServer is an easy helper method to create a new Consul
// test server with the most basic configuration.
func NewTestServer() (*TestServer, error) {
	return NewTestServerConfigT(nil, nil)
}

func NewTestServerConfig(cb ServerConfigCallback) (*TestServer, error) {
	return NewTestServerConfigT(nil, cb)
}

// NewTestServerConfig creates a new TestServer, and makes a call to an optional
// callback function to modify the configuration. If there is an error
// configuring or starting the server, the server will NOT be running when the
// function returns (thus you do not need to stop it).
func NewTestServerConfigT(t testing.TB, cb ServerConfigCallback) (*TestServer, error) {
	return newTestServerConfigT(t, cb)
}

// newTestServerConfigT is the internal helper for NewTestServerConfigT.
func newTestServerConfigT(t testing.TB, cb ServerConfigCallback) (*TestServer, error) {
	path, err := exec.LookPath("consul")
	if err != nil || path == "" {
		return nil, fmt.Errorf("consul not found on $PATH - download and install " +
			"consul or skip this test")
	}

	prefix := "consul"
	if t != nil {
		// Use test name for tmpdir if available
		prefix = strings.Replace(t.Name(), "/", "_", -1)
	}
	tmpdir, err := ioutil.TempDir("", prefix)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create tempdir")
	}

	cfg := defaultServerConfig()
	testWriter := TestWriter(t)
	cfg.Stdout = testWriter
	cfg.Stderr = testWriter

	cfg.DataDir = filepath.Join(tmpdir, "data")
	if cb != nil {
		cb(cfg)
	}

	b, err := json.Marshal(cfg)
	if err != nil {
		cfg.ReturnPorts()
		os.RemoveAll(tmpdir)
		return nil, errors.Wrap(err, "failed marshaling json")
	}

	if t != nil {
		// if you really want this output ensure to pass a valid t
		t.Logf("CONFIG JSON: %s", string(b))
	}
	configFile := filepath.Join(tmpdir, "config.json")
	if err := ioutil.WriteFile(configFile, b, 0644); err != nil {
		cfg.ReturnPorts()
		os.RemoveAll(tmpdir)
		return nil, errors.Wrap(err, "failed writing config content")
	}

	stdout := testWriter
	if cfg.Stdout != nil {
		stdout = cfg.Stdout
	}
	stderr := testWriter
	if cfg.Stderr != nil {
		stderr = cfg.Stderr
	}

	// Start the server
	args := []string{"agent", "-config-file", configFile}
	args = append(args, cfg.Args...)
	cmd := exec.Command("consul", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		cfg.ReturnPorts()
		os.RemoveAll(tmpdir)
		return nil, errors.Wrap(err, "failed starting command")
	}

	httpAddr := fmt.Sprintf("127.0.0.1:%d", cfg.Ports.HTTP)
	client := cleanhttp.DefaultClient()
	if strings.HasPrefix(cfg.Addresses.HTTP, "unix://") {
		httpAddr = cfg.Addresses.HTTP
		tr := cleanhttp.DefaultTransport()
		tr.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", httpAddr[len("unix://"):])
		}
		client = &http.Client{Transport: tr}
	}

	server := &TestServer{
		Config: cfg,
		cmd:    cmd,

		HTTPAddr:  httpAddr,
		HTTPSAddr: fmt.Sprintf("127.0.0.1:%d", cfg.Ports.HTTPS),
		LANAddr:   fmt.Sprintf("127.0.0.1:%d", cfg.Ports.SerfLan),
		WANAddr:   fmt.Sprintf("127.0.0.1:%d", cfg.Ports.SerfWan),

		HTTPClient: client,

		tmpdir: tmpdir,
	}

	// Wait for the server to be ready
	if err := server.waitForAPI(); err != nil {
		server.Stop()
		return nil, err
	}

	return server, nil
}

// Stop stops the test Consul server, and removes the Consul data
// directory once we are done.
func (s *TestServer) Stop() error {
	defer s.Config.ReturnPorts()
	defer os.RemoveAll(s.tmpdir)

	// There was no process
	if s.cmd == nil {
		return nil
	}

	if s.cmd.Process != nil {
		if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
			return errors.Wrap(err, "failed to kill consul server")
		}
	}

	// wait for the process to exit to be sure that the data dir can be
	// deleted on all platforms.
	return s.cmd.Wait()
}

// waitForAPI waits for only the agent HTTP endpoint to start
// responding. This is an indication that the agent has started,
// but will likely return before a leader is elected.
func (s *TestServer) waitForAPI() error {
	var failed bool

	// This retry replicates the logic of retry.Run to allow for nested retries.
	// By returning an error we can wrap TestServer creation with retry.Run
	// in makeClientWithConfig.
	timer := retry.TwoSeconds()
	deadline := time.Now().Add(timer.Timeout)
	for !time.Now().After(deadline) {
		time.Sleep(timer.Wait)

		resp, err := s.HTTPClient.Get(s.url("/v1/agent/self"))
		if err != nil {
			failed = true
			continue
		}
		resp.Body.Close()

		if err = s.requireOK(resp); err != nil {
			failed = true
			continue
		}
		failed = false
	}
	if failed {
		return fmt.Errorf("api unavailable")
	}
	return nil
}

// waitForLeader waits for the Consul server's HTTP API to become
// available, and then waits for a known leader and an index of
// 2 or more to be observed to confirm leader election is done.
func (s *TestServer) WaitForLeader(t *testing.T) {
	retry.Run(t, func(r *retry.R) {
		// Query the API and check the status code.
		url := s.url("/v1/catalog/nodes")
		resp, err := s.HTTPClient.Get(url)
		if err != nil {
			r.Fatalf("failed http get '%s': %v", url, err)
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			r.Fatal("failed OK response", err)
		}

		// Ensure we have a leader and a node registration.
		if leader := resp.Header.Get("X-Consul-KnownLeader"); leader != "true" {
			r.Fatalf("Consul leader status: %#v", leader)
		}
		index, err := strconv.ParseInt(resp.Header.Get("X-Consul-Index"), 10, 64)
		if err != nil {
			r.Fatal("bad consul index", err)
		}
		if index < 2 {
			r.Fatal("consul index should be at least 2")
		}
	})
}

// WaitForSerfCheck ensures we have a node with serfHealth check registered
// Behavior mirrors testrpc.WaitForTestAgent but avoids the dependency cycle in api pkg
func (s *TestServer) WaitForSerfCheck(t *testing.T) {
	retry.Run(t, func(r *retry.R) {
		// Query the API and check the status code.
		url := s.url("/v1/catalog/nodes?index=0")
		resp, err := s.HTTPClient.Get(url)
		if err != nil {
			r.Fatal("failed http get", err)
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			r.Fatal("failed OK response", err)
		}

		// Watch for the anti-entropy sync to finish.
		var payload []map[string]interface{}
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&payload); err != nil {
			r.Fatal(err)
		}
		if len(payload) < 1 {
			r.Fatal("No nodes")
		}

		// Ensure the serfHealth check is registered
		url = s.url(fmt.Sprintf("/v1/health/node/%s", payload[0]["Node"]))
		resp, err = s.HTTPClient.Get(url)
		if err != nil {
			r.Fatal("failed http get", err)
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			r.Fatal("failed OK response", err)
		}
		dec = json.NewDecoder(resp.Body)
		if err = dec.Decode(&payload); err != nil {
			r.Fatal(err)
		}

		var found bool
		for _, check := range payload {
			if check["CheckID"].(string) == "serfHealth" {
				found = true
				break
			}
		}
		if !found {
			r.Fatal("missing serfHealth registration")
		}
	})
}
