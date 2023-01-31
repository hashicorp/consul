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


	t.Run("service name is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -name flag")
	})

	t.Run("peer or partition is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-name=testservice",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Must provide -consumer-peers or -consumer-partitions flag")
	})

	t.Run("valid peer name is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-name=testservice",
			"-consumer-peers=a,",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Invalid peer")
	})

	t.Run("valid partition name is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-name=testservice",
			"-consumer-partitions=par1,",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Invalid partition")
	})

	t.Run("initial config entry should be created", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-peers=a,b",
			"-consumer-partitions=par1,par2",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), "Successfully exported service")
	})

	t.Run("existing config entry should be updated", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-peers=a,b",
			"-consumer-partitions=par1,par2",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), `Successfully exported service testservice to peers "a,b" and to partitions "par1,par2"`)

		args = []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-peers=c",
			"-consumer-partitions=par3",
		}

		code = cmd.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), `Successfully exported service testservice to peers "c" and to partitions "par3"`)
	})
}
