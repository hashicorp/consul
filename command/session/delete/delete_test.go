package delete

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSessionDelete_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSessionDelete(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	id, _, err := client.Session().CreateNoChecks(&api.SessionEntry{}, nil)
	require.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		id,
	}
	require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())

	s, _, err := client.Session().List(nil)
	require.NoError(t, err)
	require.Len(t, s, 0)
}
