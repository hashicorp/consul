package api

import (
	crand "crypto/rand"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testutil"
)

var consulConfig = `{
	"ports": {
		"dns": 19000,
		"http": 18800,
		"rpc": 18600,
		"serf_lan": 18200,
		"serf_wan": 18400,
		"server": 18000
	},
	"data_dir": "%s",
	"bootstrap": true,
	"server": true
}`

type testServer struct {
	pid        int
	dataDir    string
	configFile string
}

func (s *testServer) stop() {
	defer os.RemoveAll(s.dataDir)
	defer os.RemoveAll(s.configFile)

	cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", s.pid))
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func newTestServer(t *testing.T) *testServer {
	if path, err := exec.LookPath("consul"); err != nil || path == "" {
		t.Log("consul not found on $PATH, skipping")
		t.SkipNow()
	}

	pidFile, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	pidFile.Close()
	os.Remove(pidFile.Name())

	dataDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	configFile, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	configContent := fmt.Sprintf(consulConfig, dataDir)
	if _, err := configFile.WriteString(configContent); err != nil {
		t.Fatalf("err: %s", err)
	}
	configFile.Close()

	// Start the server
	cmd := exec.Command("consul", "agent", "-config-file", configFile.Name())
	if err := cmd.Start(); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Allow the server some time to start, and verify we have a leader.
	client := new(http.Client)
	testutil.WaitForResult(func() (bool, error) {
		resp, err := client.Get("http://127.0.0.1:18800/v1/status/leader")
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil || !strings.Contains(string(body), "18000") {
			return false, fmt.Errorf("No leader")
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})

	return &testServer{
		pid:        cmd.Process.Pid,
		dataDir:    dataDir,
		configFile: configFile.Name(),
	}
}

func makeClient(t *testing.T) (*Client, *testServer) {
	server := newTestServer(t)
	conf := DefaultConfig()
	conf.Address = "127.0.0.1:18800"
	client, err := NewClient(conf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	return client, server
}

func testKey() string {
	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		panic(fmt.Errorf("Failed to read random bytes: %v", err))
	}

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%12x",
		buf[0:4],
		buf[4:6],
		buf[6:8],
		buf[8:10],
		buf[10:16])
}

func TestSetQueryOptions(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	r := c.newRequest("GET", "/v1/kv/foo")
	q := &QueryOptions{
		Datacenter:        "foo",
		AllowStale:        true,
		RequireConsistent: true,
		WaitIndex:         1000,
		WaitTime:          100 * time.Second,
		Token:             "12345",
	}
	r.setQueryOptions(q)

	if r.params.Get("dc") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if _, ok := r.params["stale"]; !ok {
		t.Fatalf("bad: %v", r.params)
	}
	if _, ok := r.params["consistent"]; !ok {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("index") != "1000" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("wait") != "100000ms" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("token") != "12345" {
		t.Fatalf("bad: %v", r.params)
	}
}

func TestSetWriteOptions(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	r := c.newRequest("GET", "/v1/kv/foo")
	q := &WriteOptions{
		Datacenter: "foo",
		Token:      "23456",
	}
	r.setWriteOptions(q)

	if r.params.Get("dc") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("token") != "23456" {
		t.Fatalf("bad: %v", r.params)
	}
}

func TestRequestToHTTP(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	r := c.newRequest("DELETE", "/v1/kv/foo")
	q := &QueryOptions{
		Datacenter: "foo",
	}
	r.setQueryOptions(q)
	req, err := r.toHTTP()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if req.Method != "DELETE" {
		t.Fatalf("bad: %v", req)
	}
	if req.URL.String() != "http://127.0.0.1:18800/v1/kv/foo?dc=foo" {
		t.Fatalf("bad: %v", req)
	}
}

func TestParseQueryMeta(t *testing.T) {
	resp := &http.Response{
		Header: make(map[string][]string),
	}
	resp.Header.Set("X-Consul-Index", "12345")
	resp.Header.Set("X-Consul-LastContact", "80")
	resp.Header.Set("X-Consul-KnownLeader", "true")

	qm := &QueryMeta{}
	if err := parseQueryMeta(resp, qm); err != nil {
		t.Fatalf("err: %v", err)
	}

	if qm.LastIndex != 12345 {
		t.Fatalf("Bad: %v", qm)
	}
	if qm.LastContact != 80*time.Millisecond {
		t.Fatalf("Bad: %v", qm)
	}
	if !qm.KnownLeader {
		t.Fatalf("Bad: %v", qm)
	}
}
