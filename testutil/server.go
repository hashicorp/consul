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
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/testutil/retry"
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
	DNS     int `json:"dns,omitempty"`
	HTTP    int `json:"http,omitempty"`
	HTTPS   int `json:"https,omitempty"`
	SerfLan int `json:"serf_lan,omitempty"`
	SerfWan int `json:"serf_wan,omitempty"`
	Server  int `json:"server,omitempty"`

	// Deprecated
	RPC int `json:"rpc,omitempty"`
}

// TestAddressConfig contains the bind addresses for various
// components of the Consul server.
type TestAddressConfig struct {
	HTTP string `json:"http,omitempty"`
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
	DisableCheckpoint   bool                   `json:"disable_update_check"`
	LogLevel            string                 `json:"log_level,omitempty"`
	Bind                string                 `json:"bind_addr,omitempty"`
	Addresses           *TestAddressConfig     `json:"addresses,omitempty"`
	Ports               *TestPortConfig        `json:"ports,omitempty"`
	RaftProtocol        int                    `json:"raft_protocol,omitempty"`
	ACLMasterToken      string                 `json:"acl_master_token,omitempty"`
	ACLDatacenter       string                 `json:"acl_datacenter,omitempty"`
	ACLDefaultPolicy    string                 `json:"acl_default_policy,omitempty"`
	ACLEnforceVersion8  bool                   `json:"acl_enforce_version_8"`
	Encrypt             string                 `json:"encrypt,omitempty"`
	CAFile              string                 `json:"ca_file,omitempty"`
	CertFile            string                 `json:"cert_file,omitempty"`
	KeyFile             string                 `json:"key_file,omitempty"`
	VerifyIncoming      bool                   `json:"verify_incoming,omitempty"`
	VerifyIncomingRPC   bool                   `json:"verify_incoming_rpc,omitempty"`
	VerifyIncomingHTTPS bool                   `json:"verify_incoming_https,omitempty"`
	VerifyOutgoing      bool                   `json:"verify_outgoing,omitempty"`
	Stdout, Stderr      io.Writer              `json:"-"`
	Args                []string               `json:"-"`
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

	return &TestServerConfig{
		NodeName:          fmt.Sprintf("node%d", randomPort()),
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
			DNS:     randomPort(),
			HTTP:    randomPort(),
			HTTPS:   randomPort(),
			SerfLan: randomPort(),
			SerfWan: randomPort(),
			Server:  randomPort(),
			RPC:     randomPort(),
		},
	}
}

// randomPort asks the kernel for a random port to use.
func randomPort() int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
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
}

// NewTestServer is an easy helper method to create a new Consul
// test server with the most basic configuration.
func NewTestServer() (*TestServer, error) {
	return NewTestServerConfig(nil)
}

// NewTestServerConfig creates a new TestServer, and makes a call to an optional
// callback function to modify the configuration. If there is an error
// configuring or starting the server, the server will NOT be running when the
// function returns (thus you do not need to stop it).
func NewTestServerConfig(cb ServerConfigCallback) (*TestServer, error) {
	if path, err := exec.LookPath("consul"); err != nil || path == "" {
		return nil, fmt.Errorf("consul not found on $PATH - download and install " +
			"consul or skip this test")
	}

	dataDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		return nil, errors.Wrap(err, "failed creating tempdir")
	}

	configFile, err := ioutil.TempFile(dataDir, "config")
	if err != nil {
		defer os.RemoveAll(dataDir)
		return nil, errors.Wrap(err, "failed creating temp config")
	}

	consulConfig := defaultServerConfig()
	consulConfig.DataDir = dataDir

	if cb != nil {
		cb(consulConfig)
	}

	configContent, err := json.Marshal(consulConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed marshaling json")
	}

	if _, err := configFile.Write(configContent); err != nil {
		defer configFile.Close()
		defer os.RemoveAll(dataDir)
		return nil, errors.Wrap(err, "failed writing config content")
	}
	configFile.Close()

	stdout := io.Writer(os.Stdout)
	if consulConfig.Stdout != nil {
		stdout = consulConfig.Stdout
	}

	stderr := io.Writer(os.Stderr)
	if consulConfig.Stderr != nil {
		stderr = consulConfig.Stderr
	}

	// Start the server
	args := []string{"agent", "-config-file", configFile.Name()}
	args = append(args, consulConfig.Args...)
	cmd := exec.Command("consul", args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed starting command")
	}

	var httpAddr string
	var client *http.Client
	if strings.HasPrefix(consulConfig.Addresses.HTTP, "unix://") {
		httpAddr = consulConfig.Addresses.HTTP
		trans := cleanhttp.DefaultTransport()
		trans.DialContext = func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", httpAddr[7:])
		}
		client = &http.Client{
			Transport: trans,
		}
	} else {
		httpAddr = fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.HTTP)
		client = cleanhttp.DefaultClient()
	}

	server := &TestServer{
		Config: consulConfig,
		cmd:    cmd,

		HTTPAddr:  httpAddr,
		HTTPSAddr: fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.HTTPS),
		LANAddr:   fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.SerfLan),
		WANAddr:   fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.SerfWan),

		HTTPClient: client,
	}

	// Wait for the server to be ready
	var startErr error
	if consulConfig.Bootstrap {
		startErr = server.waitForLeader()
	} else {
		startErr = server.waitForAPI()
	}
	if startErr != nil {
		defer server.Stop()
		return nil, errors.Wrap(startErr, "failed waiting for server to start")
	}

	return server, nil
}

// Stop stops the test Consul server, and removes the Consul data
// directory once we are done.
func (s *TestServer) Stop() error {
	defer os.RemoveAll(s.Config.DataDir)

	if s.cmd != nil {
		if s.cmd.Process != nil {
			if err := s.cmd.Process.Kill(); err != nil {
				return errors.Wrap(err, "failed to kill consul server")
			}
		}

		// wait for the process to exit to be sure that the data dir can be
		// deleted on all platforms.
		return s.cmd.Wait()
	}

	// There was no process
	return nil
}

type failer struct {
	failed bool
}

func (f *failer) Log(args ...interface{}) { fmt.Println(args) }
func (f *failer) FailNow()                { f.failed = true }

// waitForAPI waits for only the agent HTTP endpoint to start
// responding. This is an indication that the agent has started,
// but will likely return before a leader is elected.
func (s *TestServer) waitForAPI() error {
	f := &failer{}
	retry.Run(f, func(r *retry.R) {
		resp, err := s.HTTPClient.Get(s.url("/v1/agent/self"))
		if err != nil {
			r.Fatal(err)
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			r.Fatal("failed OK respose", err)
		}
	})
	if f.failed {
		return errors.New("failed waiting for API")
	}
	return nil
}

// waitForLeader waits for the Consul server's HTTP API to become
// available, and then waits for a known leader and an index of
// 1 or more to be observed to confirm leader election is done.
// It then waits to ensure the anti-entropy sync has completed.
func (s *TestServer) waitForLeader() error {
	f := &failer{}
	timer := &retry.Timer{Timeout: 3 * time.Second, Wait: 250 * time.Millisecond}
	var index int64
	retry.RunWith(timer, f, func(r *retry.R) {
		// Query the API and check the status code.
		url := s.url(fmt.Sprintf("/v1/catalog/nodes?index=%d&wait=2s", index))
		resp, err := s.HTTPClient.Get(url)
		if err != nil {
			r.Fatal("failed http get", err)
		}
		defer resp.Body.Close()
		if err := s.requireOK(resp); err != nil {
			r.Fatal("failed OK response", err)
		}

		// Ensure we have a leader and a node registration.
		if leader := resp.Header.Get("X-Consul-KnownLeader"); leader != "true" {
			r.Fatalf("Consul leader status: %#v", leader)
		}
		index, err = strconv.ParseInt(resp.Header.Get("X-Consul-Index"), 10, 64)
		if err != nil {
			r.Fatal("bad consul index", err)
		}
		if index == 0 {
			r.Fatal("consul index is 0")
		}

		// Watch for the anti-entropy sync to finish.
		var v []map[string]interface{}
		dec := json.NewDecoder(resp.Body)
		if err := dec.Decode(&v); err != nil {
			r.Fatal(err)
		}
		if len(v) < 1 {
			r.Fatal("No nodes")
		}
		taggedAddresses, ok := v[0]["TaggedAddresses"].(map[string]interface{})
		if !ok {
			r.Fatal("Missing tagged addresses")
		}
		if _, ok := taggedAddresses["lan"]; !ok {
			r.Fatal("No lan tagged addresses")
		}
	})
	if f.failed {
		return errors.New("failed waiting for leader")
	}
	return nil
}
