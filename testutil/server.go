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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
)

// offset is used to atomically increment the port numbers.
var offset uint64

// TestPortConfig configures the various ports used for services
// provided by the Consul server.
type TestPortConfig struct {
	DNS     int `json:"dns,omitempty"`
	HTTP    int `json:"http,omitempty"`
	RPC     int `json:"rpc,omitempty"`
	SerfLan int `json:"serf_lan,omitempty"`
	SerfWan int `json:"serf_wan,omitempty"`
	Server  int `json:"server,omitempty"`
}

// TestAddressConfig contains the bind addresses for various
// components of the Consul server.
type TestAddressConfig struct {
	HTTP string `json:"http,omitempty"`
}

// TestServerConfig is the main server configuration struct.
type TestServerConfig struct {
	Bootstrap  bool               `json:"bootstrap,omitempty"`
	Server     bool               `json:"server,omitempty"`
	DataDir    string             `json:"data_dir,omitempty"`
	Datacenter string             `json:"datacenter,omitempty"`
	LogLevel   string             `json:"log_level,omitempty"`
	Addresses  *TestAddressConfig `json:"addresses,omitempty"`
	Ports      *TestPortConfig    `json:"ports,omitempty"`
}

// ServerConfigCallback is a function interface which can be
// passed to NewTestServerConfig to modify the server config.
type ServerConfigCallback func(c *TestServerConfig)

// defaultServerConfig returns a new TestServerConfig struct
// with all of the listen ports incremented by one.
func defaultServerConfig() *TestServerConfig {
	idx := int(atomic.AddUint64(&offset, 1))

	return &TestServerConfig{
		Bootstrap: true,
		Server:    true,
		LogLevel:  "debug",
		Ports: &TestPortConfig{
			DNS:     20000 + idx,
			HTTP:    21000 + idx,
			RPC:     22000 + idx,
			SerfLan: 23000 + idx,
			SerfWan: 24000 + idx,
			Server:  25000 + idx,
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

// TestServer is the main server wrapper struct.
type TestServer struct {
	PID    int
	Config *TestServerConfig
	t      *testing.T

	HTTPAddr string
	LANAddr  string
	WANAddr  string
}

// NewTestServer is an easy helper method to create a new Consul
// test server with the most basic configuration.
func NewTestServer(t *testing.T) *TestServer {
	return NewTestServerConfig(t, nil)
}

// NewTestServerConfig creates a new TestServer, and makes a call to
// an optional callback function to modify the configuration.
func NewTestServerConfig(t *testing.T, cb ServerConfigCallback) *TestServer {
	if path, err := exec.LookPath("consul"); err != nil || path == "" {
		t.Skip("consul not found on $PATH, skipping")
	}

	dataDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	configFile, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(configFile.Name())

	consulConfig := defaultServerConfig()
	consulConfig.DataDir = dataDir

	if cb != nil {
		cb(consulConfig)
	}

	configContent, err := json.Marshal(consulConfig)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if _, err := configFile.Write(configContent); err != nil {
		t.Fatalf("err: %s", err)
	}
	configFile.Close()

	// Start the server
	cmd := exec.Command("consul", "agent", "-config-file", configFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	server := &TestServer{
		Config: consulConfig,
		PID:    cmd.Process.Pid,
		t:      t,

		HTTPAddr: fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.HTTP),
		LANAddr:  fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.SerfLan),
		WANAddr:  fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.SerfWan),
	}

	// Wait for the server to be ready
	if err := server.waitForLeader(); err != nil {
		t.Fatalf("err: %s", err)
	}

	return server
}

// Stop stops the test Consul server, and removes the Consul data
// directory once we are done.
func (s *TestServer) Stop() {
	defer os.RemoveAll(s.Config.DataDir)

	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", s.PID))
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

// waitForLeader waits for the Consul server's HTTP API to become
// available, and then waits for a known leader and an index of
// 1 or more to be observed to confirm leader election is done.
func (s *TestServer) waitForLeader() error {
	url := fmt.Sprintf("http://127.0.0.1:%d/v1/catalog/nodes", s.Config.Ports.HTTP)

	WaitForResult(func() (bool, error) {
		resp, err := http.Get(url)
		if err != nil {
			return false, err
		}
		resp.Body.Close()

		// Ensure we have a leader and a node registeration
		if leader := resp.Header.Get("X-Consul-KnownLeader"); leader != "true" {
			fmt.Println(leader)
			return false, fmt.Errorf("Consul leader status: %#v", leader)
		}
		if resp.Header.Get("X-Consul-Index") == "0" {
			return false, fmt.Errorf("Consul index is 0")
		}

		return true, nil
	}, func(err error) {
		s.Stop()
		panic(err)
	})

	return nil
}

// url is a helper function which takes a relative URL and
// makes it into a proper URL against the local Consul server.
func (s *TestServer) url(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", s.Config.Ports.HTTP, path)
}

// put performs a new HTTP PUT request.
func (s *TestServer) put(path string, body io.Reader) {
	req, err := http.NewRequest("PUT", s.url(path), body)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	s.request(req)
}

// get performs a new HTTP GET request.
func (s *TestServer) get(path string) []byte {
	req, err := http.NewRequest("GET", s.url(path), nil)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	return s.request(req)
}

// request is a generic HTTP request helper to make a request and
// ensure the response code is acceptable.
func (s *TestServer) request(req *http.Request) []byte {
	// Perform the PUT
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}

	// Check status code
	if resp.StatusCode != 200 {
		s.t.Fatalf("Bad response code: %d\nBody:\n%s", resp.StatusCode, body)
	}

	return body
}

// encodePayload returns a new io.Reader wrapping the encoded contents
// of the payload, suitable for passing directly to a new request.
func (s *TestServer) encodePayload(payload interface{}) io.Reader {
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	if err := enc.Encode(payload); err != nil {
		s.t.Fatalf("err: %s", err)
	}
	return &encoded
}

// JoinLAN is used to join nodes within the same datacenter.
func (s *TestServer) JoinLAN(addr string) {
	s.get("/v1/agent/join/" + addr)
}

// JoinWAN is used to join remote datacenters together.
func (s *TestServer) JoinWAN(addr string) {
	s.get("/v1/agent/join/" + addr + "?wan=1")
}

// SetKV sets an individual key in the K/V store.
func (s *TestServer) SetKV(key string, val []byte) {
	s.put("/v1/kv/"+key, bytes.NewBuffer(val))
}

// PopulateKV fills the Consul KV with data from a generic map.
func (s *TestServer) PopulateKV(data map[string][]byte) {
	for k, v := range data {
		s.SetKV(k, v)
	}
}

// AddService adds a new service to the Consul instance. It also
// automatically adds a health check with the given status, which
// can be one of "passing", "warning", or "critical".
func (s *TestServer) AddService(name, status string, tags []string) {
	svc := &TestService{
		Name: name,
		Tags: tags,
	}
	payload := s.encodePayload(svc)
	s.put("/v1/agent/service/register", payload)

	chkName := "service:" + name
	chk := &TestCheck{
		Name:      chkName,
		ServiceID: name,
		TTL:       "10m",
	}
	payload = s.encodePayload(chk)
	s.put("/v1/agent/check/register", payload)

	switch status {
	case "passing":
		s.put("/v1/agent/check/pass/"+chkName, nil)
	case "warning":
		s.put("/v1/agent/check/warn/"+chkName, nil)
	case "critical":
		s.put("/v1/agent/check/fail/"+chkName, nil)
	default:
		s.t.Fatalf("Unrecognized status: %s", status)
	}
}

// AddCheck adds a check to the Consul instance. If the serviceID is
// left empty (""), then the check will be associated with the node.
// The check status may be "passing", "warning", or "critical".
func (s *TestServer) AddCheck(name, serviceID, status string) {
	chk := &TestCheck{
		ID:   name,
		Name: name,
		TTL:  "10m",
	}
	if serviceID != "" {
		chk.ServiceID = serviceID
	}

	payload := s.encodePayload(chk)
	s.put("/v1/agent/check/register", payload)

	switch status {
	case "passing":
		s.put("/v1/agent/check/pass/"+name, nil)
	case "warning":
		s.put("/v1/agent/check/warn/"+name, nil)
	case "critical":
		s.put("/v1/agent/check/fail/"+name, nil)
	default:
		s.t.Fatalf("Unrecognized status: %s", status)
	}
}
