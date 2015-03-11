package testutil

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

var offset uint64

type TestPortConfig struct {
	DNS     int `json:"dns,omitempty"`
	HTTP    int `json:"http,omitempty"`
	RPC     int `json:"rpc,omitempty"`
	SerfLan int `json:"serf_lan,omitempty"`
	SerfWan int `json:"serf_wan,omitempty"`
	Server  int `json:"server,omitempty"`
}

type TestAddressConfig struct {
	HTTP string `json:"http,omitempty"`
}

type TestServerConfig struct {
	Bootstrap bool               `json:"bootstrap,omitempty"`
	Server    bool               `json:"server,omitempty"`
	DataDir   string             `json:"data_dir,omitempty"`
	LogLevel  string             `json:"log_level,omitempty"`
	Addresses *TestAddressConfig `json:"addresses,omitempty"`
	Ports     *TestPortConfig    `json:"ports,omitempty"`
}

type ServerConfigCallback func(c *TestServerConfig)

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

type TestService struct {
	ID      string   `json:",omitempty"`
	Name    string   `json:",omitempty"`
	Tags    []string `json:",omitempty"`
	Address string   `json:",omitempty"`
	Port    int      `json:",omitempty"`
}

type TestCheck struct {
	ID        string `json:",omitempty"`
	Name      string `json:",omitempty"`
	ServiceID string `json:",omitempty"`
	TTL       string `json:",omitempty"`
}

type TestServer struct {
	PID     int
	Config  *TestServerConfig
	APIAddr string
	t       *testing.T
}

func NewTestServer(t *testing.T) *TestServer {
	return NewTestServerConfig(t, nil)
}

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
		Config:  consulConfig,
		PID:     cmd.Process.Pid,
		APIAddr: fmt.Sprintf("127.0.0.1:%d", consulConfig.Ports.HTTP),
		t:       t,
	}

	// Wait for the server to be ready
	if err := server.waitForLeader(); err != nil {
		t.Fatalf("err: %s", err)
	}

	return server
}

func (s *TestServer) Stop() {
	defer os.RemoveAll(s.Config.DataDir)

	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", s.PID))
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

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

func (s *TestServer) url(path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", s.Config.Ports.HTTP, path)
}

func (s *TestServer) put(path string, body io.Reader) {
	req, err := http.NewRequest("PUT", s.url(path), body)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	s.request(req)
}

func (s *TestServer) delete(path string) {
	var body io.Reader
	req, err := http.NewRequest("DELETE", s.url(path), body)
	if err != nil {
		s.t.Fatalf("err: %s", err)
	}
	s.request(req)
}

func (s *TestServer) request(req *http.Request) {
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
}

func (s *TestServer) encodePayload(payload interface{}) io.Reader {
	var encoded bytes.Buffer
	enc := json.NewEncoder(&encoded)
	if err := enc.Encode(payload); err != nil {
		s.t.Fatalf("err: %s", err)
	}
	return &encoded
}

func (s *TestServer) KVSet(key string, val []byte) {
	s.put("/v1/kv/"+key, bytes.NewBuffer(val))
}

func (s *TestServer) KVDelete(key string) {
	s.delete("/v1/kv/" + key)
}

func (s *TestServer) AddService(name, status string, tags []string) {
	svc := &TestService{
		Name: name,
		Tags: tags,
	}
	payload := s.encodePayload(svc)
	s.put("/v1/agent/service/register", payload)

	chk := &TestCheck{
		Name:      name,
		ServiceID: name,
		TTL:       "10m",
	}
	payload = s.encodePayload(chk)
	s.put("/v1/agent/check/register", payload)

	switch status {
	case "passing":
		s.put("/v1/agent/check/pass/"+name, nil)
	case "warning":
		s.put("/v1/agent/check/warn/"+name, nil)
	case "failing":
		s.put("/v1/agent/check/fail/"+name, nil)
	}
}
