// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package renew

import (
	"crypto/x509"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/testutil"

	caCreate "github.com/hashicorp/consul/command/tls/ca/create"
	certCreate "github.com/hashicorp/consul/command/tls/cert/create"
)

func TestValidateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTlsCertRenewCommand_InvalidArgs(t *testing.T) {
	t.Parallel()

	type testcase struct {
		args      []string
		expectErr string
	}

	cases := map[string]testcase{
		"no args (ca/key inferred)": {[]string{},
			"Please provide the existingcert like #DOMAIN#-agent-#.pem that has to be renewed."},
		"no ca": {[]string{"-ca", "", "-key", ""},
			"Please provide the ca"},
		"no key": {[]string{"-ca", "foo.pem", "-key", ""},
			"Please provide the key"},
		"no existingcert": {[]string{"-existingcert", ""},
			"Please provide the existingcert like #DOMAIN#-agent-#.pem that has to be renewed."},
		"no existingkey": {[]string{"-existingcert", "foo.pem", "-existingkey", ""},
			"Please provide the existingkey like #DOMAIN#-agent-#-key.pem."},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ui := cli.NewMockUi()
			cmd := New(ui)
			require.NotEqual(t, 0, cmd.Run(tc.args))
			got := ui.ErrorWriter.String()
			if tc.expectErr == "" {
				require.NotEmpty(t, got) // don't care
			} else {
				require.Contains(t, got, tc.expectErr)
			}
		})
	}
}

func TestTlsCertRenewCommand_fileCreate(t *testing.T) {
	testDir := testutil.TempDir(t, "tlsrenew")

	defer switchToTempDir(t, testDir)()

	// Setup CA keys
	createCA(t, "consul")
	createCA(t, "nomad")

	// Setup Certs that will be renewed
	createCert(t, "dc1", "consul", "server", "0", "")
	createCert(t, "dc1", "consul", "server", "1", "mysrv")
	createCert(t, "dc2", "nomad", "server", "0", "")
	createCert(t, "dc1", "consul", "client", "0", "")
	createCert(t, "dc1", "consul", "client", "1", "")
	createCert(t, "dc2", "nomad", "client", "0", "")
	createCert(t, "dc1", "consul", "cli", "0", "")
	createCert(t, "dc1", "consul", "cli", "1", "")
	createCert(t, "dc2", "nomad", "cli", "0", "")

	type testcase struct {
		name           string
		typ            string
		args           []string
		expectCertPath string
		expectCN       string
		expectDNS      []string
		expectIP       []net.IP
	}

	// The following subtests must run serially.
	cases := []testcase{
		{"server0",
			"server",
			[]string{"-existingcert", "dc1-server-consul-0.pem", "-existingkey", "dc1-server-consul-0-key.pem"},
			"dc1-server-consul-2.pem",
			"server.dc1.consul",
			[]string{
				"server.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"server1-with-node",
			"server",
			[]string{"-existingcert", "dc1-server-consul-1.pem", "-existingkey", "dc1-server-consul-1-key.pem"},
			"dc1-server-consul-3.pem",
			"server.dc1.consul",
			[]string{
				"mysrv.server.dc1.consul",
				"server.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"server0-dc2-altdomain",
			"server",
			[]string{"-existingcert", "dc2-server-nomad-0.pem", "-existingkey", "dc2-server-nomad-0-key.pem"},
			"dc2-server-nomad-1.pem",
			"server.dc2.nomad",
			[]string{
				"server.dc2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client0",
			"client",
			[]string{"-existingcert", "dc1-client-consul-0.pem", "-existingkey", "dc1-client-consul-0-key.pem"},
			"dc1-client-consul-2.pem",
			"client.dc1.consul",
			[]string{
				"client.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client1",
			"client",
			[]string{"-existingcert", "dc1-client-consul-1.pem", "-existingkey", "dc1-client-consul-1-key.pem"},
			"dc1-client-consul-3.pem",
			"client.dc1.consul",
			[]string{
				"client.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client0-dc2-altdomain",
			"client",
			[]string{"-existingcert", "dc2-client-nomad-0.pem", "-existingkey", "dc2-client-nomad-0-key.pem"},
			"dc2-client-nomad-1.pem",
			"client.dc2.nomad",
			[]string{
				"client.dc2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"cli0",
			"cli",
			[]string{"-existingcert", "dc1-cli-consul-0.pem", "-existingkey", "dc1-cli-consul-0-key.pem"},
			"dc1-cli-consul-2.pem",
			"cli.dc1.consul",
			[]string{
				"cli.dc1.consul",
				"localhost",
			},
			nil,
		},
		{"cli1",
			"cli",
			[]string{"-existingcert", "dc1-cli-consul-1.pem", "-existingkey", "dc1-cli-consul-1-key.pem"},
			"dc1-cli-consul-3.pem",
			"cli.dc1.consul",
			[]string{
				"cli.dc1.consul",
				"localhost",
			},
			nil,
		},
		{"cli0-dc2-altdomain",
			"cli",
			[]string{"-existingcert", "dc2-cli-nomad-0.pem", "-existingkey", "dc2-cli-nomad-0-key.pem"},
			"dc2-cli-nomad-1.pem",
			"cli.dc2.nomad",
			[]string{
				"cli.dc2.nomad",
				"localhost",
			},
			nil,
		},
	}

	for _, tc := range cases {
		tc := tc
		require.True(t, t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := New(ui)
			require.Equal(t, 0, cmd.Run(tc.args))
			require.Equal(t, "", ui.ErrorWriter.String())

			cert := expectFiles(t, tc.expectCertPath)
			require.Equal(t, tc.expectCN, cert.Subject.CommonName)
			require.True(t, cert.BasicConstraintsValid)
			require.Equal(t, x509.KeyUsageDigitalSignature|x509.KeyUsageKeyEncipherment, cert.KeyUsage)
			switch tc.typ {
			case "server":
				require.Equal(t,
					[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
					cert.ExtKeyUsage)
			case "client":
				require.Equal(t,
					[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
					cert.ExtKeyUsage)
			case "cli":
				require.Len(t, cert.ExtKeyUsage, 0)
			}
			require.False(t, cert.IsCA)
			require.Equal(t, tc.expectDNS, cert.DNSNames)
			require.Equal(t, tc.expectIP, cert.IPAddresses)
		}))
	}
}

func expectFiles(t *testing.T, certPath string) *x509.Certificate {
	t.Helper()

	require.FileExists(t, certPath)

	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)

	cert, err := connect.ParseCert(string(certData))
	require.NoError(t, err)
	require.NotNil(t, cert)

	return cert
}

func createCA(t *testing.T, domain string) {
	t.Helper()

	ui := cli.NewMockUi()
	caCmd := caCreate.New(ui)

	args := []string{
		"-domain=" + domain,
	}

	require.Equal(t, 0, caCmd.Run(args))
	require.Equal(t, "", ui.ErrorWriter.String())

	require.FileExists(t, "consul-agent-ca.pem")
}

func createCert(t *testing.T, dc string, domain string, agent string, num string, node string) {
	t.Helper()

	ui := cli.NewMockUi()
	caCmd := certCreate.New(ui)

	args := []string{
		"-" + agent, "-dc=" + dc, "-domain=" + domain, "-ca=" + domain + "-agent-ca.pem", "-key=" + domain + "-agent-ca-key.pem", "-node=" + node,
	}

	require.Equal(t, 0, caCmd.Run(args))
	require.Equal(t, "", ui.ErrorWriter.String())

	require.FileExists(t, dc+"-"+agent+"-"+domain+"-"+num+".pem")
	require.FileExists(t, dc+"-"+agent+"-"+domain+"-"+num+"-key.pem")
}

// switchToTempDir is meant to be used in a defer statement like:
//
//	defer switchToTempDir(t, testDir)()
//
// This exploits the fact that the body of a defer is evaluated
// EXCEPT for the final function call invocation inline with the code
// where it is found. Only the final evaluation happens in the defer
// at a later time. In this case it means we switch to the temp
// directory immediately and defer switching back in one line of test
// code.
func switchToTempDir(t *testing.T, testDir string) func() {
	previousDirectory, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(testDir))
	return func() {
		os.Chdir(previousDirectory)
	}
}
