package export

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"
)

func TestExportCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestExportCommand(t *testing.T) {

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = a.Shutdown() })
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	//client := a.Client()

	//acceptingClient := acceptor.Client()
	//dialingClient := dialer.Client()


	t.Run("service is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{

		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -service flag")

	})

	t.Run("peer is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-service=testservice",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -peers flag")

	})

	t.Run("valid peer name is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-service=testservice",
			"-peers=a,",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Invalid peer")

	})

	t.Run("initial config entry should be created", func(t *testing.T) {

		
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=testservice",
			"-peers=a,b",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), "Successfully exported service")
	})

	t.Run("existing config entry should be updated", func(t *testing.T) {
		
	})
}
