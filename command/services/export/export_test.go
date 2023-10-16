// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package export

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"
)

func TestExportCommand(t *testing.T) {

	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	t.Run("help output should have no tabs", func(t *testing.T) {
		if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
			t.Fatal("help has tabs")
		}
	})

	a := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = a.Shutdown() })
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	t.Run("peer or partition is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-name=testservice",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -consumer-peers or -consumer-partitions flag")
	})
	t.Run("service name is required", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -name flag")
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
			"-consumer-partitions=a,",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Invalid partition")
	})

	t.Run("initial config entry should be created w/ partitions", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-partitions=a,b",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), "Successfully exported service")
	})

	t.Run("initial config entry should be created w/ peers", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-peers=a,b",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), "Successfully exported service")
	})

	t.Run("existing config entry should be updated w/ new peers and partitions", func(t *testing.T) {

		ui := cli.NewMockUi()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-peers=a,b",
		}

		code := New(ui).Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), `Successfully exported service "testservice" to cluster peers "a,b"`)

		args = []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-peers=c",
		}

		code = New(ui).Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), `Successfully exported service "testservice" to cluster peers "c"`)

		args = []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=testservice",
			"-consumer-partitions=d",
		}

		code = New(ui).Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(), `Successfully exported service "testservice" to partitions "d"`)
	})
}
