// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/sdk/testutil"
)

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
