package test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	capi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
)

// Holds a running server and connected client.
// Plus anscillary data useful in testing is available on embedded TestServer.
type TestConsulServer struct {
	*testutil.TestServer
	client *capi.Client
}

// NewTestConsulServer runs the Consul binary given on the path and returning
// TestConsulServer object which encaptulates the server and a connected client.
func NewTestConsulServer(t *testing.T, path string) TestConsulServer {
	certFile, err := filepath.Abs("./testdata/cert.pem")
	if err != nil {
		t.Fatalf("failed to open test cert file: %v\n", err)
	}
	keyFile, err := filepath.Abs("./testdata/key.pem")
	if err != nil {
		t.Fatalf("failed to open test key file: %v\n", err)
	}
	consul, err := testutil.NewTestServerConfigPathT(t, path,
		func(c *testutil.TestServerConfig) {
			c.LogLevel = "warn"
			c.Stdout = ioutil.Discard
			c.Stderr = ioutil.Discard
			c.CertFile = certFile
			c.KeyFile = keyFile
			if strings.Contains(path, "1.13") {
				c.Ports.GRPCTLS = 0
			}
		})
	if err != nil {
		t.Fatalf("failed to start consul server: %v", err)
	}

	client, err := capi.NewClient(&capi.Config{
		Address: "http://" + consul.HTTPAddr,
	})
	if err != nil {
		t.Fatalf("failed to create consul client: %v", err)
	}

	return TestConsulServer{consul, client}
}

func (s TestConsulServer) Client() *capi.Client {
	return s.client
}

func caConf(addr, token, rootPath, intrPath string) *capi.CAConfig {
	return &capi.CAConfig{
		Provider: "vault",
		Config: map[string]any{
			"Address":             addr,
			"Token":               token,
			"RootPKIPath":         rootPath,
			"IntermediatePKIPath": intrPath,
			"LeafCertTTL":         "72h",
			"RotationPeriod":      "2160h",
			"IntermediateCertTTL": "8760h",
		},
	}
}
