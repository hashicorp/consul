package renew

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestSessionRenewCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestSessionRenew_noTTL(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui)

	cases := map[string]struct {
		se *api.SessionEntry
	}{
		"no-ttl": {
			se: &api.SessionEntry{},
		},
		"ttl": {
			&api.SessionEntry{
				TTL: "30s",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			id, _, err := client.Session().CreateNoChecks(tc.se, nil)
			require.NoError(t, err)

			args := []string{
				"-http-addr=" + a.HTTPAddr(),
				id,
			}
			require.Equal(t, 0, c.Run(args), ui.ErrorWriter.String())
		})
	}
}
