package create

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/command/tls/ca/create"
)

func TestValidateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTlsCertCreateCommand_fileCreate(t *testing.T) {
	require := require.New(t)

	previousDirectory, err := os.Getwd()
	require.NoError(err)

	testDir := testutil.TempDir(t, "tls")
	defer os.RemoveAll(testDir)
	defer os.Chdir(previousDirectory)

	os.Chdir(testDir)

	ui := cli.NewMockUi()
	cmd := New(ui)

	// Setup CA keys
	createCA(t, "consul")

	caPath := path.Join(testDir, "consul-agent-ca.pem")
	require.FileExists(caPath)

	args := []string{
		"-server",
	}

	require.Equal(0, cmd.Run(args))
	require.Equal("", ui.ErrorWriter.String())

	certPath := path.Join(testDir, "dc1-server-consul-0.pem")
	keyPath := path.Join(testDir, "dc1-server-consul-0-key.pem")

	require.FileExists(certPath)
	require.FileExists(keyPath)

	certData, err := ioutil.ReadFile(certPath)
	require.NoError(err)
	keyData, err := ioutil.ReadFile(keyPath)
	require.NoError(err)

	cert, err := connect.ParseCert(string(certData))
	require.NoError(err)
	require.NotNil(cert)

	signer, err := connect.ParseSigner(string(keyData))
	require.NoError(err)
	require.NotNil(signer)

	// TODO - maybe we should validate some certs here.
}

func createCA(t *testing.T, domain string) {
	ui := cli.NewMockUi()
	caCmd := create.New(ui)

	args := []string{
		"-domain=" + domain,
	}

	require.Equal(t, 0, caCmd.Run(args))
	require.Equal(t, "", ui.ErrorWriter.String())
}
