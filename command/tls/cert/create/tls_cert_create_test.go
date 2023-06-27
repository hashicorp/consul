// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package create

import (
	"crypto"
	"crypto/x509"
	"io/fs"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/connect"
	caCreate "github.com/hashicorp/consul/command/tls/ca/create"
	"github.com/hashicorp/consul/sdk/testutil"
)

func TestValidateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTlsCertCreateCommand_InvalidArgs(t *testing.T) {
	t.Parallel()

	type testcase struct {
		args      []string
		expectErr string
	}

	cases := map[string]testcase{
		"no args (ca/key inferred)": {[]string{},
			"Please provide either -server, -client, or -cli"},
		"no ca": {[]string{"-ca", "", "-key", ""},
			"Please provide the ca"},
		"no key": {[]string{"-ca", "foo.pem", "-key", ""},
			"Please provide the key"},

		"server+client+cli": {[]string{"-server", "-client", "-cli"},
			"Please provide either -server, -client, or -cli"},
		"server+client": {[]string{"-server", "-client"},
			"Please provide either -server, -client, or -cli"},
		"server+cli": {[]string{"-server", "-cli"},
			"Please provide either -server, -client, or -cli"},
		"client+cli": {[]string{"-client", "-cli"},
			"Please provide either -server, -client, or -cli"},

		"client+node": {[]string{"-client", "-node", "foo"},
			"-node requires -server"},
		"cli+node": {[]string{"-cli", "-node", "foo"},
			"-node requires -server"},
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

func TestTlsCertCreateCommand_fileCreate(t *testing.T) {
	testDir := testutil.TempDir(t, "tls")

	defer switchToTempDir(t, testDir)()

	// Setup CA keys
	createCA(t, "consul")
	createCA(t, "nomad")

	type testcase struct {
		name      string
		typ       string
		args      []string
		certPath  string
		keyPath   string
		expectCN  string
		expectDNS []string
		expectIP  []net.IP
	}

	// The following subtests must run serially.
	cases := []testcase{
		{"server0",
			"server",
			[]string{"-server"},
			"dc1-server-consul-0.pem",
			"dc1-server-consul-0-key.pem",
			"server.dc1.consul",
			[]string{
				"server.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"server1-with-node",
			"server",
			[]string{"-server", "-node", "mysrv"},
			"dc1-server-consul-1.pem",
			"dc1-server-consul-1-key.pem",
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
			[]string{"-server", "-dc", "dc2", "-domain", "nomad"},
			"dc2-server-nomad-0.pem",
			"dc2-server-nomad-0-key.pem",
			"server.dc2.nomad",
			[]string{
				"server.dc2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client0",
			"client",
			[]string{"-client"},
			"dc1-client-consul-0.pem",
			"dc1-client-consul-0-key.pem",
			"client.dc1.consul",
			[]string{
				"client.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client1",
			"client",
			[]string{"-client"},
			"dc1-client-consul-1.pem",
			"dc1-client-consul-1-key.pem",
			"client.dc1.consul",
			[]string{
				"client.dc1.consul",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"client0-dc2-altdomain",
			"client",
			[]string{"-client", "-dc", "dc2", "-domain", "nomad"},
			"dc2-client-nomad-0.pem",
			"dc2-client-nomad-0-key.pem",
			"client.dc2.nomad",
			[]string{
				"client.dc2.nomad",
				"localhost",
			},
			[]net.IP{{127, 0, 0, 1}},
		},
		{"cli0",
			"cli",
			[]string{"-cli"},
			"dc1-cli-consul-0.pem",
			"dc1-cli-consul-0-key.pem",
			"cli.dc1.consul",
			[]string{
				"cli.dc1.consul",
				"localhost",
			},
			nil,
		},
		{"cli1",
			"cli",
			[]string{"-cli"},
			"dc1-cli-consul-1.pem",
			"dc1-cli-consul-1-key.pem",
			"cli.dc1.consul",
			[]string{
				"cli.dc1.consul",
				"localhost",
			},
			nil,
		},
		{"cli0-dc2-altdomain",
			"cli",
			[]string{"-cli", "-dc", "dc2", "-domain", "nomad"},
			"dc2-cli-nomad-0.pem",
			"dc2-cli-nomad-0-key.pem",
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

			cert, _ := expectFiles(t, tc.certPath, tc.keyPath)
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

func expectFiles(t *testing.T, certPath, keyPath string) (*x509.Certificate, crypto.Signer) {
	t.Helper()

	require.FileExists(t, certPath)
	require.FileExists(t, keyPath)

	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal("should not happen", err)
	}
	if want, have := fs.FileMode(0600), fi.Mode().Perm(); want != have {
		t.Fatalf("private key file %s: permissions: want: %o; have: %o", keyPath, want, have)
	}

	certData, err := os.ReadFile(certPath)
	require.NoError(t, err)
	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	cert, err := connect.ParseCert(string(certData))
	require.NoError(t, err)
	require.NotNil(t, cert)

	signer, err := connect.ParseSigner(string(keyData))
	require.NoError(t, err)
	require.NotNil(t, signer)

	return cert, signer
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
