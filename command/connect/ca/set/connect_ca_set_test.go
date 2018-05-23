package set

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/cli"
)

func TestConnectCASetConfigCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestConnectCASetConfigCommand(t *testing.T) {
	t.Parallel()
	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-config-file=test-fixtures/ca_config.json",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	req := structs.DCSpecificRequest{
		Datacenter: "dc1",
	}
	var reply structs.CAConfiguration
	require.NoError(a.RPC("ConnectCA.ConfigurationGet", &req, &reply))
	require.Equal("consul", reply.Provider)

	parsed, err := ca.ParseConsulCAConfig(reply.Config)
	require.NoError(err)
	require.Equal(24*time.Hour, parsed.RotationPeriod)
}
