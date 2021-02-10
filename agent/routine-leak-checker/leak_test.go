package leakcheck

import (
	"crypto/x509"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func testTLSCertificates(serverName string) (cert string, key string, cacert string, err error) {
	ca, _, err := tlsutil.GenerateCA(tlsutil.CAOpts{})
	if err != nil {
		return "", "", "", err
	}

	// generate leaf
	serial, err := tlsutil.GenerateSerialNumber()
	if err != nil {
		return "", "", "", err
	}

	signer, _, err := tlsutil.GeneratePrivateKey()
	if err != nil {
		return "", "", "", err
	}

	cert, privateKey, err := tlsutil.GenerateCert(
		signer,
		ca,
		serial,
		"Test Cert Name",
		365,
		[]string{serverName},
		nil,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	)
	if err != nil {
		return "", "", "", err
	}

	return cert, privateKey, ca, nil
}

func setupPrimaryServer(t *testing.T) *agent.TestAgent {
	d := testutil.TempDir(t, "leaks-primary-server")

	certPEM, keyPEM, caPEM, err := testTLSCertificates("server.primary.consul")
	require.NoError(t, err)

	certPath := filepath.Join(d, "cert.pem")
	keyPath := filepath.Join(d, "key.pem")
	caPath := filepath.Join(d, "cacert.pem")

	require.NoError(t, ioutil.WriteFile(certPath, []byte(certPEM), 0600))
	require.NoError(t, ioutil.WriteFile(keyPath, []byte(keyPEM), 0600))
	require.NoError(t, ioutil.WriteFile(caPath, []byte(caPEM), 0600))

	aclParams := agent.DefaulTestACLConfigParams()
	aclParams.PrimaryDatacenter = "primary"
	aclParams.EnableTokenReplication = true

	config := `
	   server = true
		datacenter = "primary"
		primary_datacenter = "primary"
		
		connect {
			enabled = true
		}
		
		auto_encrypt {
			allow_tls = true
		}
	` + agent.TestACLConfigWithParams(aclParams)

	a := agent.NewTestAgent(t, config)
	t.Cleanup(func() { a.Shutdown() })

	testrpc.WaitForTestAgent(t, a.RPC, "primary", testrpc.WithToken(agent.TestDefaultMasterToken))

	return a
}

func TestTestAgentLeaks_Server(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	/*
		Eventually go routine leak checking should be moved into other packages such as the agent
		and agent/consul packages. However there are too many leaks for the test to run properly.

		Many of the leaks are due to blocking queries from clients to servers being uncancellable.
		Until we can move away from net/rpc and fix some of the other issues we don't want a
		completely unbounded test which is guaranteed to fail 100% of the time. For now this
		test will do. When we do update it we should add this in a *_test.go file in the packages
		that we want to enable leak checking within:

		import (
			"testing"

			"go.uber.org/goleak"
		)

		func TestMain(m *testing.M) {
			goleak.VerifyTestMain(m,
				goleak.IgnoreTopFunction("k8s.io/klog.(*loggingT).flushDaemon"),
				goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
				goleak.IgnoreTopFunction("github.com/hashicorp/consul/sdk/freeport.checkFreedPorts"),
			)
		}
	*/

	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("k8s.io/klog.(*loggingT).flushDaemon"),
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("github.com/hashicorp/consul/sdk/freeport.checkFreedPorts"),
	)

	primaryServer := setupPrimaryServer(t)
	primaryServer.Shutdown()
}
