// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	crand "crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
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

// Makes a client with Audit enabled, it requires ACLs
func makeAuditClient(t *testing.T) (*Client, *testutil.TestServer) {
	return makeClientWithConfig(t, func(clientConfig *Config) {
		clientConfig.Token = "root"
	}, func(serverConfig *testutil.TestServerConfig) {
		serverConfig.PrimaryDatacenter = "dc1"
		serverConfig.ACL.Tokens.InitialManagement = "root"
		serverConfig.ACL.Tokens.Agent = "root"
		serverConfig.ACL.Enabled = true
		serverConfig.ACL.DefaultPolicy = "deny"
		serverConfig.Audit = &testutil.TestAuditConfig{
			Enabled: true,
		}
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

func TestAPI_DefaultConfig_env(t *testing.T) {
	// t.Parallel() // DO NOT ENABLE !!!
	// do not enable t.Parallel for this test since it modifies global state
	// (environment) which has non-deterministic effects on the other tests
	// which derive their default configuration from the environment

	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	addr := "1.2.3.4:5678"
	token := "abcd1234"
	auth := "username:password"

	os.Setenv(HTTPAddrEnvName, addr)
	defer os.Setenv(HTTPAddrEnvName, "")
	os.Setenv(HTTPTokenEnvName, token)
	defer os.Setenv(HTTPTokenEnvName, "")
	os.Setenv(HTTPAuthEnvName, auth)
	defer os.Setenv(HTTPAuthEnvName, "")
	os.Setenv(HTTPSSLEnvName, "1")
	defer os.Setenv(HTTPSSLEnvName, "")
	os.Setenv(HTTPCAFile, "ca.pem")
	defer os.Setenv(HTTPCAFile, "")
	os.Setenv(HTTPCAPath, "certs/")
	defer os.Setenv(HTTPCAPath, "")
	os.Setenv(HTTPClientCert, "client.crt")
	defer os.Setenv(HTTPClientCert, "")
	os.Setenv(HTTPClientKey, "client.key")
	defer os.Setenv(HTTPClientKey, "")
	os.Setenv(HTTPTLSServerName, "consul.test")
	defer os.Setenv(HTTPTLSServerName, "")
	os.Setenv(HTTPSSLVerifyEnvName, "0")
	defer os.Setenv(HTTPSSLVerifyEnvName, "")

	for i, config := range []*Config{
		DefaultConfig(),
		DefaultConfigWithLogger(testutil.Logger(t)),
		DefaultNonPooledConfig(),
	} {
		if config.Address != addr {
			t.Errorf("expected %q to be %q", config.Address, addr)
		}
		if config.Token != token {
			t.Errorf("expected %q to be %q", config.Token, token)
		}
		if config.HttpAuth == nil {
			t.Fatalf("expected HttpAuth to be enabled")
		}
		if config.HttpAuth.Username != "username" {
			t.Errorf("expected %q to be %q", config.HttpAuth.Username, "username")
		}
		if config.HttpAuth.Password != "password" {
			t.Errorf("expected %q to be %q", config.HttpAuth.Password, "password")
		}
		if config.Scheme != "https" {
			t.Errorf("expected %q to be %q", config.Scheme, "https")
		}
		if config.TLSConfig.CAFile != "ca.pem" {
			t.Errorf("expected %q to be %q", config.TLSConfig.CAFile, "ca.pem")
		}
		if config.TLSConfig.CAPath != "certs/" {
			t.Errorf("expected %q to be %q", config.TLSConfig.CAPath, "certs/")
		}
		if config.TLSConfig.CertFile != "client.crt" {
			t.Errorf("expected %q to be %q", config.TLSConfig.CertFile, "client.crt")
		}
		if config.TLSConfig.KeyFile != "client.key" {
			t.Errorf("expected %q to be %q", config.TLSConfig.KeyFile, "client.key")
		}
		if config.TLSConfig.Address != "consul.test" {
			t.Errorf("expected %q to be %q", config.TLSConfig.Address, "consul.test")
		}
		if !config.TLSConfig.InsecureSkipVerify {
			t.Errorf("expected SSL verification to be off")
		}

		// Use keep alives as a check for whether pooling is on or off.
		if pooled := i != 2; pooled {
			if config.Transport.DisableKeepAlives != false {
				t.Errorf("expected keep alives to be enabled")
			}
		} else {
			if config.Transport.DisableKeepAlives != true {
				t.Errorf("expected keep alives to be disabled")
			}
		}
	}
}

func TestAPI_SetupTLSConfig(t *testing.T) {
	// if this test is failing because of expired certificates
	// use the procedure in test/CA-GENERATION.md
	t.Parallel()
	// A default config should result in a clean default client config.
	tlsConfig := &TLSConfig{}
	cc, err := SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected := &tls.Config{RootCAs: cc.RootCAs}
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: \n%v, \n%v", cc, expected)
	}

	// Try some address variations with and without ports.
	tlsConfig.Address = "127.0.0.1"
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected.ServerName = "127.0.0.1"
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: %v", cc)
	}

	tlsConfig.Address = "127.0.0.1:80"
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected.ServerName = "127.0.0.1"
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: %v", cc)
	}

	tlsConfig.Address = "demo.consul.io:80"
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected.ServerName = "demo.consul.io"
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: %v", cc)
	}

	tlsConfig.Address = "[2001:db8:a0b:12f0::1]"
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected.ServerName = "[2001:db8:a0b:12f0::1]"
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: %v", cc)
	}

	tlsConfig.Address = "[2001:db8:a0b:12f0::1]:80"
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected.ServerName = "2001:db8:a0b:12f0::1"
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: %v", cc)
	}

	// Skip verification.
	tlsConfig.InsecureSkipVerify = true
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expected.InsecureSkipVerify = true
	if !reflect.DeepEqual(cc, expected) {
		t.Fatalf("bad: %v", cc)
	}

	// Make a new config that hits all the file parsers.
	tlsConfig = &TLSConfig{
		CertFile: "../test/hostname/Alice.crt",
		KeyFile:  "../test/hostname/Alice.key",
		CAFile:   "../test/hostname/CertAuth.crt",
	}
	cc, err = SetupTLSConfig(tlsConfig)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cc.Certificates) != 1 {
		t.Fatalf("missing certificate: %v", cc.Certificates)
	}
	if cc.RootCAs == nil {
		t.Fatalf("didn't load root CAs")
	}

	// Use a directory to load the certs instead
	cc, err = SetupTLSConfig(&TLSConfig{
		CAPath: "../test/ca_path",
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	expectedCaPoolByDir := getExpectedCaPoolByDir(t)
	assertDeepEqual(t, expectedCaPoolByDir, cc.RootCAs, cmpCertPool)

	// Load certs in-memory
	certPEM, err := os.ReadFile("../test/hostname/Alice.crt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	keyPEM, err := os.ReadFile("../test/hostname/Alice.key")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	caPEM, err := os.ReadFile("../test/hostname/CertAuth.crt")
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Setup config with in-memory certs
	cc, err = SetupTLSConfig(&TLSConfig{
		CertPEM: certPEM,
		KeyPEM:  keyPEM,
		CAPem:   caPEM,
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(cc.Certificates) != 1 {
		t.Fatalf("missing certificate: %v", cc.Certificates)
	}
	if cc.RootCAs == nil {
		t.Fatalf("didn't load root CAs")
	}
}

func TestAPI_ClientTLSOptions(t *testing.T) {
	t.Parallel()
	// Start a server that verifies incoming HTTPS connections
	_, srvVerify := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.CAFile = "../test/client_certs/rootca.crt"
		conf.CertFile = "../test/client_certs/server.crt"
		conf.KeyFile = "../test/client_certs/server.key"
		conf.VerifyIncomingHTTPS = true
	})
	defer srvVerify.Stop()

	// Start a server without VerifyIncomingHTTPS
	_, srvNoVerify := makeClientWithConfig(t, nil, func(conf *testutil.TestServerConfig) {
		conf.CAFile = "../test/client_certs/rootca.crt"
		conf.CertFile = "../test/client_certs/server.crt"
		conf.KeyFile = "../test/client_certs/server.key"
		conf.VerifyIncomingHTTPS = false
	})
	defer srvNoVerify.Stop()

	// Client without a cert
	t.Run("client without cert, validation", func(t *testing.T) {
		client, err := NewClient(&Config{
			Address: srvVerify.HTTPSAddr,
			Scheme:  "https",
			TLSConfig: TLSConfig{
				Address: "consul.test",
				CAFile:  "../test/client_certs/rootca.crt",
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Should fail
		_, err = client.Agent().Self()
		if err == nil || !strings.Contains(err.Error(), "bad certificate") {
			t.Fatal(err)
		}
	})

	// Client with a valid cert
	t.Run("client with cert, validation", func(t *testing.T) {
		client, err := NewClient(&Config{
			Address: srvVerify.HTTPSAddr,
			Scheme:  "https",
			TLSConfig: TLSConfig{
				Address:  "consul.test",
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Should succeed
		_, err = client.Agent().Self()
		if err != nil {
			t.Fatal(err)
		}
	})

	// Client without a cert
	t.Run("client without cert, no validation", func(t *testing.T) {
		client, err := NewClient(&Config{
			Address: srvNoVerify.HTTPSAddr,
			Scheme:  "https",
			TLSConfig: TLSConfig{
				Address: "consul.test",
				CAFile:  "../test/client_certs/rootca.crt",
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Should succeed
		_, err = client.Agent().Self()
		if err != nil {
			t.Fatal(err)
		}
	})

	// Client with a valid cert
	t.Run("client with cert, no validation", func(t *testing.T) {
		client, err := NewClient(&Config{
			Address: srvNoVerify.HTTPSAddr,
			Scheme:  "https",
			TLSConfig: TLSConfig{
				Address:  "consul.test",
				CAFile:   "../test/client_certs/rootca.crt",
				CertFile: "../test/client_certs/client.crt",
				KeyFile:  "../test/client_certs/client.key",
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Should succeed
		_, err = client.Agent().Self()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestAPI_SetQueryOptions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	r := c.newRequest("GET", "/v1/kv/foo")
	q := &QueryOptions{
		Namespace:         "operator",
		Partition:         "asdf",
		Datacenter:        "foo",
		Peer:              "dc10",
		AllowStale:        true,
		RequireConsistent: true,
		WaitIndex:         1000,
		WaitTime:          100 * time.Second,
		Token:             "12345",
		Near:              "nodex",
		LocalOnly:         true,
	}
	r.setQueryOptions(q)

	if r.params.Get("ns") != "operator" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("partition") != "asdf" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("peer") != "dc10" {
		t.Fatalf("bad: %v", r.params)
	}
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
	if r.header.Get("X-Consul-Token") != "12345" {
		t.Fatalf("bad: %v", r.header)
	}
	if r.params.Get("near") != "nodex" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("local-only") != "true" {
		t.Fatalf("bad: %v", r.params)
	}
	assert.Equal(t, "", r.header.Get("Cache-Control"))

	r = c.newRequest("GET", "/v1/kv/foo")
	q = &QueryOptions{
		UseCache:     true,
		MaxAge:       30 * time.Second,
		StaleIfError: 345678 * time.Millisecond, // Fractional seconds should be rounded
	}
	r.setQueryOptions(q)

	_, ok := r.params["cached"]
	assert.True(t, ok)
	assert.Equal(t, "max-age=30, stale-if-error=346", r.header.Get("Cache-Control"))
}

func TestAPI_SetWriteOptions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	r := c.newRequest("GET", "/v1/kv/foo")
	q := &WriteOptions{
		Namespace:  "operator",
		Partition:  "asdf",
		Datacenter: "foo",
		Token:      "23456",
	}
	r.setWriteOptions(q)
	if r.params.Get("ns") != "operator" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("partition") != "asdf" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.params.Get("dc") != "foo" {
		t.Fatalf("bad: %v", r.params)
	}
	if r.header.Get("X-Consul-Token") != "23456" {
		t.Fatalf("bad: %v", r.header)
	}
}

func TestAPI_Headers(t *testing.T) {
	t.Parallel()

	var request *http.Request
	c, s := makeClientWithConfig(t, func(c *Config) {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = func(r *http.Request) (*url.URL, error) {
			// Keep track of the last request sent
			request = r
			return nil, nil
		}
		c.Transport = transport
	}, nil)
	defer s.Stop()

	if len(c.Headers()) != 0 {
		t.Fatalf("expected headers to be empty: %v", c.Headers())
	}

	c.AddHeader("Hello", "World")
	r := c.newRequest("GET", "/v1/kv/foo")

	if r.header.Get("Hello") != "World" {
		t.Fatalf("Hello header not set : %v", r.header)
	}

	c.SetHeaders(http.Header{
		"Auth": []string{"Token"},
	})

	r = c.newRequest("GET", "/v1/kv/foo")
	if r.header.Get("Hello") != "" {
		t.Fatalf("Hello header should not be set: %v", r.header)
	}

	if r.header.Get("Auth") != "Token" {
		t.Fatalf("Auth header not set: %v", r.header)
	}

	kv := c.KV()
	_, err := kv.Put(&KVPair{Key: "test-headers", Value: []byte("foo")}, nil)
	require.NoError(t, err)
	require.Equal(t, "application/octet-stream", request.Header.Get("Content-Type"))

	_, _, err = kv.Get("test-headers", nil)
	require.NoError(t, err)
	require.Equal(t, "", request.Header.Get("Content-Type"))

	_, err = kv.Delete("test-headers", nil)
	require.NoError(t, err)
	require.Equal(t, "", request.Header.Get("Content-Type"))

	err = c.Snapshot().Restore(nil, strings.NewReader("foo"))
	require.Error(t, err)
	require.Equal(t, "application/octet-stream", request.Header.Get("Content-Type"))

	_, _, err = c.Event().Fire(&UserEvent{
		Name:    "test",
		Payload: []byte("foo"),
	}, nil)
	require.NoError(t, err)
	require.Equal(t, "application/octet-stream", request.Header.Get("Content-Type"))
}

func TestAPI_Deprecated(t *testing.T) {
	t.Parallel()
	c, s := makeClientWithConfig(t, func(c *Config) {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		c.Transport = transport
	}, nil)
	defer s.Stop()
	// Rules translation functionality was completely removed in Consul 1.15.
	_, err := c.ACL().RulesTranslate(strings.NewReader(`
	agent "" {
	  policy = "read"
	}
	`))
	require.Error(t, err)
	_, err = c.ACL().RulesTranslateToken("")
	require.Error(t, err)
}

func TestAPI_RequestToHTTP(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

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
	if req.URL.RequestURI() != "/v1/kv/foo?dc=foo" {
		t.Fatalf("bad: %v", req)
	}
}

func TestAPI_RequestToHTTP_PrefixedWithSlashes(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	c.config.Address = "/tmp/mysocket.sock"
	r := c.newRequest("DELETE", "/v1/kv/foo")
	q := &QueryOptions{
		Datacenter: "foo",
	}
	r.setQueryOptions(q)
	req, err := r.toHTTP()
	require.NoError(t, err)
	// validate that socket communications that do not use the host, detect
	// slashes in the host name and replace it with local host.
	// this is required since go started validating req.host in 1.20.6.
	// prior to that they would strip out the slahes for you.  They removed that
	// behavior and added more strict validation as part of a CVE.
	// https://github.com/golang/go/issues/11206
	require.Equal(t, "localhost", req.Host)

}

func TestAPI_ParseQueryMeta(t *testing.T) {
	t.Parallel()
	resp := &http.Response{
		Header: make(map[string][]string),
	}
	resp.Header.Set("X-Consul-Index", "12345")
	resp.Header.Set("X-Consul-LastContact", "80")
	resp.Header.Set("X-Consul-KnownLeader", "true")
	resp.Header.Set("X-Consul-Translate-Addresses", "true")
	resp.Header.Set("X-Consul-Default-ACL-Policy", "deny")
	resp.Header.Set("X-Consul-Results-Filtered-By-ACLs", "true")

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
	if !qm.AddressTranslationEnabled {
		t.Fatalf("Bad: %v", qm)
	}
	if qm.DefaultACLPolicy != "deny" {
		t.Fatalf("Bad: %v", qm)
	}
	if !qm.ResultsFilteredByACLs {
		t.Fatalf("Bad: %v", qm)
	}
}

func TestAPI_UnixSocket(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	tempDir := testutil.TempDir(t, "consul")
	socket := filepath.Join(tempDir, "test.sock")

	c, s := makeClientWithConfig(t, func(c *Config) {
		c.Address = "unix://" + socket
	}, func(c *testutil.TestServerConfig) {
		c.Addresses = &testutil.TestAddressConfig{
			HTTP: "unix://" + socket,
		}
	})
	defer s.Stop()

	agent := c.Agent()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if info["Config"]["NodeName"].(string) == "" {
		t.Fatalf("bad: %v", info)
	}
}

func TestAPI_durToMsec(t *testing.T) {
	t.Parallel()
	if ms := durToMsec(0); ms != "0ms" {
		t.Fatalf("bad: %s", ms)
	}

	if ms := durToMsec(time.Millisecond); ms != "1ms" {
		t.Fatalf("bad: %s", ms)
	}

	if ms := durToMsec(time.Microsecond); ms != "1ms" {
		t.Fatalf("bad: %s", ms)
	}

	if ms := durToMsec(5 * time.Millisecond); ms != "5ms" {
		t.Fatalf("bad: %s", ms)
	}
}

func TestAPI_IsRetryableError(t *testing.T) {
	t.Parallel()
	if IsRetryableError(nil) {
		t.Fatal("should not be a retryable error")
	}

	if IsRetryableError(fmt.Errorf("not the error you are looking for")) {
		t.Fatal("should not be a retryable error")
	}

	if !IsRetryableError(fmt.Errorf(serverError)) {
		t.Fatal("should be a retryable error")
	}

	if !IsRetryableError(&net.OpError{Err: fmt.Errorf("network conn error")}) {
		t.Fatal("should be a retryable error")
	}
}

func TestAPI_GenerateEnv(t *testing.T) {
	t.Parallel()

	c := &Config{
		Address:   "127.0.0.1:8500",
		Token:     "test",
		TokenFile: "test.file",
		Scheme:    "http",
		TLSConfig: TLSConfig{
			CAFile:             "",
			CAPath:             "",
			CertFile:           "",
			KeyFile:            "",
			Address:            "",
			InsecureSkipVerify: true,
		},
	}

	expected := []string{
		"CONSUL_HTTP_ADDR=127.0.0.1:8500",
		"CONSUL_HTTP_TOKEN=test",
		"CONSUL_HTTP_TOKEN_FILE=test.file",
		"CONSUL_HTTP_SSL=false",
		"CONSUL_CACERT=",
		"CONSUL_CAPATH=",
		"CONSUL_CLIENT_CERT=",
		"CONSUL_CLIENT_KEY=",
		"CONSUL_TLS_SERVER_NAME=",
		"CONSUL_HTTP_SSL_VERIFY=false",
		"CONSUL_HTTP_AUTH=",
	}

	require.Equal(t, expected, c.GenerateEnv())
}

func TestAPI_GenerateEnvHTTPS(t *testing.T) {
	t.Parallel()

	c := &Config{
		Address:   "127.0.0.1:8500",
		Token:     "test",
		TokenFile: "test.file",
		Scheme:    "https",
		TLSConfig: TLSConfig{
			CAFile:             "/var/consul/ca.crt",
			CAPath:             "/var/consul/ca.dir",
			CertFile:           "/var/consul/server.crt",
			KeyFile:            "/var/consul/ssl/server.key",
			Address:            "127.0.0.1:8500",
			InsecureSkipVerify: false,
		},
		HttpAuth: &HttpBasicAuth{
			Username: "user",
			Password: "password",
		},
	}

	expected := []string{
		"CONSUL_HTTP_ADDR=127.0.0.1:8500",
		"CONSUL_HTTP_TOKEN=test",
		"CONSUL_HTTP_TOKEN_FILE=test.file",
		"CONSUL_HTTP_SSL=true",
		"CONSUL_CACERT=/var/consul/ca.crt",
		"CONSUL_CAPATH=/var/consul/ca.dir",
		"CONSUL_CLIENT_CERT=/var/consul/server.crt",
		"CONSUL_CLIENT_KEY=/var/consul/ssl/server.key",
		"CONSUL_TLS_SERVER_NAME=127.0.0.1:8500",
		"CONSUL_HTTP_SSL_VERIFY=true",
		"CONSUL_HTTP_AUTH=user:password",
	}

	require.Equal(t, expected, c.GenerateEnv())
}

// TestAPI_PrefixPath() validates that Config.Address is split into
// Config.Address and Config.PathPrefix as expected.  If we want to add end to
// end testing in the future this will require configuring and running an
// API gateway / reverse proxy (e.g. nginx)
func TestAPI_PrefixPath(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		addr         string
		expectAddr   string
		expectPrefix string
	}{
		{
			name:         "with http and prefix",
			addr:         "http://reverse.proxy.com/consul/path/prefix",
			expectAddr:   "reverse.proxy.com",
			expectPrefix: "/consul/path/prefix",
		},
		{
			name:         "with https and prefix",
			addr:         "https://reverse.proxy.com/consul/path/prefix",
			expectAddr:   "reverse.proxy.com",
			expectPrefix: "/consul/path/prefix",
		},
		{
			name:         "with http and no prefix",
			addr:         "http://localhost",
			expectAddr:   "localhost",
			expectPrefix: "",
		},
		{
			name:         "with https and no prefix",
			addr:         "https://localhost",
			expectAddr:   "localhost",
			expectPrefix: "",
		},
		{
			name:         "no scheme and no prefix",
			addr:         "localhost",
			expectAddr:   "localhost",
			expectPrefix: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{Address: tc.addr}
			client, err := NewClient(c)
			require.NoError(t, err)
			require.Equal(t, tc.expectAddr, client.config.Address)
			require.Equal(t, tc.expectPrefix, client.config.PathPrefix)
		})
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
