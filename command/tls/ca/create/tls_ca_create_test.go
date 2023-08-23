package create

import (
	"crypto"
	"crypto/x509"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestValidateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestCACreateCommand(t *testing.T) {
	testDir := testutil.TempDir(t, "ca-create")

	defer switchToTempDir(t, testDir)()

	type testcase struct {
		name       string
		args       []string
		caPath     string
		keyPath    string
		extraCheck func(t *testing.T, cert *x509.Certificate)
	}
	// The following subtests must run serially.
	cases := []testcase{
		{"ca defaults",
			nil,
			"consul-agent-ca.pem",
			"consul-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Equal(t, 1825*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
				require.False(t, cert.PermittedDNSDomainsCritical)
				require.Len(t, cert.PermittedDNSDomains, 0)
			},
		},
		{"ca options",
			[]string{
				"-days=365",
				"-name-constraint=true",
				"-domain=foo",
				"-additional-name-constraint=bar",
			},
			"foo-agent-ca.pem",
			"foo-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Equal(t, 365*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
				require.True(t, cert.PermittedDNSDomainsCritical)
				require.Len(t, cert.PermittedDNSDomains, 3)
				require.ElementsMatch(t, cert.PermittedDNSDomains, []string{"foo", "localhost", "bar"})
			},
		},
		{"with cluster-id",
			[]string{
				"-domain=foo",
				"-cluster-id=uuid",
			},
			"foo-agent-ca.pem",
			"foo-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Len(t, cert.URIs, 1)
				require.Equal(t, cert.URIs[0].String(), "spiffe://uuid.foo")
			},
		},
		{"with common-name",
			[]string{
				"-common-name=foo",
			},
			"consul-agent-ca.pem",
			"consul-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.Equal(t, cert.Subject.CommonName, "foo")
			},
		},
		{"without common-name",
			[]string{},
			"consul-agent-ca.pem",
			"consul-agent-ca-key.pem",
			func(t *testing.T, cert *x509.Certificate) {
				require.True(t, strings.HasPrefix(cert.Subject.CommonName, "Consul Agent CA"))
			},
		},
	}
	for _, tc := range cases {
		tc := tc
		require.True(t, t.Run(tc.name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := New(ui)
			require.Equal(t, 0, cmd.Run(tc.args), ui.ErrorWriter.String())
			require.Equal(t, "", ui.ErrorWriter.String())

			cert, _ := expectFiles(t, tc.caPath, tc.keyPath)
			require.True(t, cert.BasicConstraintsValid)
			require.Equal(t, x509.KeyUsageCertSign|x509.KeyUsageCRLSign|x509.KeyUsageDigitalSignature, cert.KeyUsage)
			require.True(t, cert.IsCA)
			require.Equal(t, cert.AuthorityKeyId, cert.SubjectKeyId)
			tc.extraCheck(t, cert)
			require.NoError(t, os.Remove(tc.caPath))
			require.NoError(t, os.Remove(tc.keyPath))
		}))
	}

}

func expectFiles(t *testing.T, caPath, keyPath string) (*x509.Certificate, crypto.Signer) {
	t.Helper()

	require.FileExists(t, caPath)
	require.FileExists(t, keyPath)

	fi, err := os.Stat(keyPath)
	if err != nil {
		t.Fatal("should not happen", err)
	}
	if want, have := fs.FileMode(0600), fi.Mode().Perm(); want != have {
		t.Fatalf("private key file %s: permissions: want: %o; have: %o", keyPath, want, have)
	}

	caData, err := os.ReadFile(caPath)
	require.NoError(t, err)
	keyData, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	ca, err := connect.ParseCert(string(caData))
	require.NoError(t, err)
	require.NotNil(t, ca)

	signer, err := connect.ParseSigner(string(keyData))
	require.NoError(t, err)
	require.NotNil(t, signer)

	return ca, signer
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
