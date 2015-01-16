package api

import (
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"testing"
)

func TestStatusLeaderTCP(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	status := c.Status()

	leader, err := status.Leader()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("Expected leader")
	}
}

func TestStatusLeaderUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempdir, err := ioutil.TempDir("", "consul-test-")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tempdir)
	socket := fmt.Sprintf("unix://%s/test.sock", tempdir)

	clientConfig := func(c *Config) {
		c.Address = socket
	}

	serverConfig := func(c *testServerConfig) {
		if c.Addresses == nil {
			c.Addresses = &testAddressConfig{}
		}
		c.Addresses.HTTP = socket
	}

	c, s := makeClientWithConfig(t, clientConfig, serverConfig)
	defer s.stop()

	status := c.Status()

	leader, err := status.Leader()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if leader == "" {
		t.Fatalf("Expected leader")
	}
}

func TestStatusPeers(t *testing.T) {
	c, s := makeClient(t)
	defer s.stop()

	status := c.Status()

	peers, err := status.Peers()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(peers) == 0 {
		t.Fatalf("Expected peers ")
	}
}
