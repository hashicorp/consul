// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	crand "crypto/rand"
	"crypto/x509"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type configCallback func(c *Config)

func makeClient(t *testing.T) (*Client, *testutil.TestServer) {
	return makeClientWithConfig(t, nil, nil)
}

func makeClientWithoutConnect(t *testing.T) (*Client, *testutil.TestServer) {
	return makeClientWithConfig(t, nil, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.Connect = nil
	})
}

func makeACLClient(t *testing.T) (*Client, *testutil.TestServer) {
	return makeClientWithConfig(t, func(clientConfig *Config) {
		clientConfig.Token = "root"
	}, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.PrimaryDatacenter = "dc1"
		serverConfig.ACL.Tokens.InitialManagement = "root"
		serverConfig.ACL.Tokens.Agent = "root"
		serverConfig.ACL.Enabled = true
		serverConfig.ACL.DefaultPolicy = "deny"
	})
}

func makeNonBootstrappedACLClient(t *testing.T, defaultPolicy string) (*Client, *testutil.TestServer) {
	return makeClientWithConfig(t,
		func(clientConfig *Config) {
			clientConfig.Token = ""
		},
		func(serverConfig *testutil.TestServerConfig) {
			serverConfig.PrimaryDatacenter = "dc1"
			serverConfig.ACL.Enabled = true
			serverConfig.ACL.DefaultPolicy = defaultPolicy
			serverConfig.Bootstrap = true
		})
}

func makeClientWithCA(t *testing.T) (*Client, *testutil.TestServer) {
	return makeClientWithConfig(t,
		func(c *Config) {
			c.TLSConfig = TLSConfig{
				Address:  "consul.test",
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			}
		},
		func(c *testutil.TestServerConfig) {
			c.CAFile = "../test/client_certs/rootca.crt"
			c.CertFile = "../test/client_certs/server.crt"
			c.KeyFile = "../test/client_certs/server.key"
		})
}

func makeClientWithConfig(
	t *testing.T,
	cb1 configCallback,
	cb2 testutil.ServerConfigCallback) (*Client, *testutil.TestServer) {
	// Skip test when -short flag provided; any tests that create a test
	// server will take at least 100ms which is undesirable for -short
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// Make client config
	conf := DefaultConfig()
	if cb1 != nil {
		cb1(conf)
	}

	// Create server
	var server *testutil.TestServer
	var err error
	retry.RunWith(retry.ThreeTimes(), t, func(r *retry.R) {
		server, err = testutil.NewTestServerConfigT(t, cb2)
		if err != nil {
			r.Fatalf("Failed to start server: %v", err.Error())
		}
	})
	if server.Config.Bootstrap {
		server.WaitForLeader(t)
	}
	connectEnabled := server.Config.Connect["enabled"]
	if enabled, ok := connectEnabled.(bool); ok && server.Config.Server && enabled {
		server.WaitForActiveCARoot(t)
	}

	conf.Address = server.HTTPAddr

	// Create client
	client, err := NewClient(conf)
	if err != nil {
		server.Stop()
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

func testNodeServiceCheckRegistrations(t *testing.T, client *Client, datacenter string) {
	t.Helper()

	registrations := map[string]*CatalogRegistration{
		"Node foo": {
			Datacenter: datacenter,
			Node:       "foo",
			ID:         "e0155642-135d-4739-9853-a1ee6c9f945b",
			Address:    "127.0.0.2",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.2",
				"wan": "198.18.0.2",
			},
			NodeMeta: map[string]string{
				"env": "production",
				"os":  "linux",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:    "foo",
					CheckID: "foo:alive",
					Name:    "foo-liveness",
					Status:  HealthPassing,
					Notes:   "foo is alive and well",
				},
				&HealthCheck{
					Node:    "foo",
					CheckID: "foo:ssh",
					Name:    "foo-remote-ssh",
					Status:  HealthPassing,
					Notes:   "foo has ssh access",
				},
			},
			Locality: &Locality{Region: "us-west-1", Zone: "us-west-1a"},
		},
		"Service redis v1 on foo": {
			Datacenter:     datacenter,
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:     ServiceKindTypical,
				ID:       "redisV1",
				Service:  "redis",
				Tags:     []string{"v1"},
				Meta:     map[string]string{"version": "1"},
				Port:     1234,
				Address:  "198.18.1.2",
				Locality: &Locality{Region: "us-west-1", Zone: "us-west-1a"},
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "foo",
					CheckID:     "foo:redisV1",
					Name:        "redis-liveness",
					Status:      HealthPassing,
					Notes:       "redis v1 is alive and well",
					ServiceID:   "redisV1",
					ServiceName: "redis",
				},
			},
		},
		"Service redis v2 on foo": {
			Datacenter:     datacenter,
			Node:           "foo",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "redisV2",
				Service: "redis",
				Tags:    []string{"v2"},
				Meta:    map[string]string{"version": "2"},
				Port:    1235,
				Address: "198.18.1.2",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "foo",
					CheckID:     "foo:redisV2",
					Name:        "redis-v2-liveness",
					Status:      HealthPassing,
					Notes:       "redis v2 is alive and well",
					ServiceID:   "redisV2",
					ServiceName: "redis",
				},
			},
		},
		"Node bar": {
			Datacenter: datacenter,
			Node:       "bar",
			ID:         "c6e7a976-8f4f-44b5-bdd3-631be7e8ecac",
			Address:    "127.0.0.3",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.3",
				"wan": "198.18.0.3",
			},
			NodeMeta: map[string]string{
				"env": "production",
				"os":  "windows",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:    "bar",
					CheckID: "bar:alive",
					Name:    "bar-liveness",
					Status:  HealthPassing,
					Notes:   "bar is alive and well",
				},
			},
		},
		"Service redis v1 on bar": {
			Datacenter:     datacenter,
			Node:           "bar",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "redisV1",
				Service: "redis",
				Tags:    []string{"v1"},
				Meta:    map[string]string{"version": "1"},
				Port:    1234,
				Address: "198.18.1.3",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "bar",
					CheckID:     "bar:redisV1",
					Name:        "redis-liveness",
					Status:      HealthPassing,
					Notes:       "redis v1 is alive and well",
					ServiceID:   "redisV1",
					ServiceName: "redis",
				},
			},
		},
		"Service web v1 on bar": {
			Datacenter:     datacenter,
			Node:           "bar",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "webV1",
				Service: "web",
				Tags:    []string{"v1", "connect"},
				Meta:    map[string]string{"version": "1", "connect": "enabled"},
				Port:    443,
				Address: "198.18.1.4",
				Connect: &AgentServiceConnect{Native: true},
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "bar",
					CheckID:     "bar:web:v1",
					Name:        "web-v1-liveness",
					Status:      HealthPassing,
					Notes:       "web connect v1 is alive and well",
					ServiceID:   "webV1",
					ServiceName: "web",
				},
			},
		},
		"Node baz": {
			Datacenter: datacenter,
			Node:       "baz",
			ID:         "12f96b27-a7b0-47bd-add7-044a2bfc7bfb",
			Address:    "127.0.0.4",
			TaggedAddresses: map[string]string{
				"lan": "127.0.0.4",
			},
			NodeMeta: map[string]string{
				"env": "qa",
				"os":  "linux",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:    "baz",
					CheckID: "baz:alive",
					Name:    "baz-liveness",
					Status:  HealthPassing,
					Notes:   "baz is alive and well",
				},
				&HealthCheck{
					Node:    "baz",
					CheckID: "baz:ssh",
					Name:    "baz-remote-ssh",
					Status:  HealthPassing,
					Notes:   "baz has ssh access",
				},
			},
		},
		"Service web v1 on baz": {
			Datacenter:     datacenter,
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "webV1",
				Service: "web",
				Tags:    []string{"v1", "connect"},
				Meta:    map[string]string{"version": "1", "connect": "enabled"},
				Port:    443,
				Address: "198.18.1.4",
				Connect: &AgentServiceConnect{Native: true},
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "baz",
					CheckID:     "baz:web:v1",
					Name:        "web-v1-liveness",
					Status:      HealthPassing,
					Notes:       "web connect v1 is alive and well",
					ServiceID:   "webV1",
					ServiceName: "web",
				},
			},
		},
		"Service web v2 on baz": {
			Datacenter:     datacenter,
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "webV2",
				Service: "web",
				Tags:    []string{"v2", "connect"},
				Meta:    map[string]string{"version": "2", "connect": "enabled"},
				Port:    8443,
				Address: "198.18.1.4",
				Connect: &AgentServiceConnect{Native: true},
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "baz",
					CheckID:     "baz:web:v2",
					Name:        "web-v2-liveness",
					Status:      HealthPassing,
					Notes:       "web connect v2 is alive and well",
					ServiceID:   "webV2",
					ServiceName: "web",
				},
			},
		},
		"Service critical on baz": {
			Datacenter:     datacenter,
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "criticalV2",
				Service: "critical",
				Tags:    []string{"v2"},
				Meta:    map[string]string{"version": "2"},
				Port:    8080,
				Address: "198.18.1.4",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "baz",
					CheckID:     "baz:critical:v2",
					Name:        "critical-v2-liveness",
					Status:      HealthCritical,
					Notes:       "critical v2 is in the critical state",
					ServiceID:   "criticalV2",
					ServiceName: "critical",
				},
			},
		},
		"Service warning on baz": {
			Datacenter:     datacenter,
			Node:           "baz",
			SkipNodeUpdate: true,
			Service: &AgentService{
				Kind:    ServiceKindTypical,
				ID:      "warningV2",
				Service: "warning",
				Tags:    []string{"v2"},
				Meta:    map[string]string{"version": "2"},
				Port:    8081,
				Address: "198.18.1.4",
			},
			Checks: HealthChecks{
				&HealthCheck{
					Node:        "baz",
					CheckID:     "baz:warning:v2",
					Name:        "warning-v2-liveness",
					Status:      HealthWarning,
					Notes:       "warning v2 is in the warning state",
					ServiceID:   "warningV2",
					ServiceName: "warning",
				},
			},
		},
	}

	catalog := client.Catalog()
	for name, reg := range registrations {
		_, err := catalog.Register(reg, nil)
		require.NoError(t, err, "Failed catalog registration for %q: %v", name, err)
	}
}

func getExpectedCaPoolByDir(t *testing.T) *x509.CertPool {
	pool := x509.NewCertPool()
	entries, err := os.ReadDir("../test/ca_path")
	require.NoError(t, err)

	for _, entry := range entries {
		filename := path.Join("../test/ca_path", entry.Name())

		data, err := os.ReadFile(filename)
		require.NoError(t, err)

		if !pool.AppendCertsFromPEM(data) {
			t.Fatalf("could not add test ca %s to pool", filename)
		}
	}

	return pool
}

// lazyCerts has a func field which can't be compared.
var cmpCertPool = cmp.Options{
	cmpopts.IgnoreFields(x509.CertPool{}, "lazyCerts"),
	cmp.AllowUnexported(x509.CertPool{}),
}

func assertDeepEqual(t *testing.T, x, y interface{}, opts ...cmp.Option) {
	t.Helper()
	if diff := cmp.Diff(x, y, opts...); diff != "" {
		t.Fatalf("assertion failed: values are not equal\n--- expected\n+++ actual\n%v", diff)
	}
}
