package create

import (
	"io/ioutil"
	"os"
	"path"
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
	require := require.New(t)

	previousDirectory, err := os.Getwd()
	require.NoError(err)

	testDir := testutil.TempDir(t, "ca-create")

	defer os.RemoveAll(testDir)
	defer os.Chdir(previousDirectory)

	os.Chdir(testDir)

	ui := cli.NewMockUi()
	cmd := New(ui)

	require.Equal(0, cmd.Run(nil), "ca create should exit 0")

	errOutput := ui.ErrorWriter.String()
	require.Equal("", errOutput)

	caPem := path.Join(testDir, "consul-agent-ca.pem")
	require.FileExists(caPem)

	certData, err := ioutil.ReadFile(caPem)
	require.NoError(err)

	cert, err := connect.ParseCert(string(certData))
	require.NoError(err)
	require.NotNil(cert)

	require.Equal(1825*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
	require.False(cert.PermittedDNSDomainsCritical)
	require.Len(cert.PermittedDNSDomains, 0)
}

func TestCACreateCommandWithOptions(t *testing.T) {
	require := require.New(t)

	previousDirectory, err := os.Getwd()
	require.NoError(err)

	testDir := testutil.TempDir(t, "ca-create")

	defer os.RemoveAll(testDir)
	defer os.Chdir(previousDirectory)

	os.Chdir(testDir)

	ui := cli.NewMockUi()
	cmd := New(ui)

	args := []string{
		"-days=365",
		"-name-constraint=true",
		"-domain=foo",
		"-additional-name-constraint=bar",
	}

	require.Equal(0, cmd.Run(args), "ca create should exit 0")

	errOutput := ui.ErrorWriter.String()
	require.Equal("", errOutput)

	caPem := path.Join(testDir, "foo-agent-ca.pem")
	require.FileExists(caPem)

	certData, err := ioutil.ReadFile(caPem)
	require.NoError(err)

	cert, err := connect.ParseCert(string(certData))
	require.NoError(err)
	require.NotNil(cert)

	require.Equal(365*24*time.Hour, time.Until(cert.NotAfter).Round(24*time.Hour))
	require.True(cert.PermittedDNSDomainsCritical)
	require.Len(cert.PermittedDNSDomains, 3)
	require.ElementsMatch(cert.PermittedDNSDomains, []string{"foo", "localhost", "bar"})
}
