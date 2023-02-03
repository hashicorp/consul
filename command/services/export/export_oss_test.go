//go:build !consulent
// +build !consulent

package export

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"
)

func TestExportCommandOSS(t *testing.T) {

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = a.Shutdown() })
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("peer is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-name=testservice",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -consumer-peers flag")
	})
}
