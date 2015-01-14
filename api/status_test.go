package api

import (
	"io/ioutil"
	"os/user"
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
		t.Fatal("Could not create a working directory")
	}

	socket := "unix://" + tempdir + "/unix-http-test.sock"

	clientConfig := func(c *Config) {
		c.Address = socket
	}

	serverConfig := func(c *TestServerConfig) {
		user, err := user.Current()
		if err != nil {
			t.Fatal("Could not get current user")
		}

		if c.Addresses == nil {
			c.Addresses = &TestAddressConfig{}
		}
		c.Addresses.HTTP = socket + ";" + user.Uid + ";" + user.Gid + ";640"
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
