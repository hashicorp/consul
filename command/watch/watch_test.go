// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package watch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestWatchCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi(), nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestWatchCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-type=nodes"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestWatchCommand_loadToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	a := agent.NewTestAgent(t, ` `)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	const testToken = "4a0db5a1-869f-4602-ae8a-b0306a82f1ef"
	testDir := testutil.TempDir(t, "watchtest")

	fullname := filepath.Join(testDir, "token.txt")
	require.NoError(t, os.WriteFile(fullname, []byte(testToken), 0600))

	resetEnv := func() {
		os.Unsetenv("CONSUL_HTTP_TOKEN")
		os.Unsetenv("CONSUL_HTTP_TOKEN_FILE")
	}

	t.Run("token arg", func(t *testing.T) {
		resetEnv()
		defer resetEnv()

		ui := cli.NewMockUi()
		c := New(ui, nil)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-type=nodes",
			"-token", testToken,
		}

		require.NoError(t, c.flags.Parse(args))

		tok, err := c.loadToken()
		require.NoError(t, err)
		require.Equal(t, testToken, tok)
	})

	t.Run("token env", func(t *testing.T) {
		resetEnv()
		defer resetEnv()

		ui := cli.NewMockUi()
		c := New(ui, nil)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-type=nodes",
		}
		os.Setenv("CONSUL_HTTP_TOKEN", testToken)

		require.NoError(t, c.flags.Parse(args))

		tok, err := c.loadToken()
		require.NoError(t, err)
		require.Equal(t, testToken, tok)
	})

	t.Run("token file arg", func(t *testing.T) {
		resetEnv()
		defer resetEnv()

		ui := cli.NewMockUi()
		c := New(ui, nil)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-type=nodes",
			"-token-file", fullname,
		}

		require.NoError(t, c.flags.Parse(args))

		tok, err := c.loadToken()
		require.NoError(t, err)
		require.Equal(t, testToken, tok)
	})

	t.Run("token file env", func(t *testing.T) {
		resetEnv()
		defer resetEnv()

		ui := cli.NewMockUi()
		c := New(ui, nil)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-type=nodes",
		}
		os.Setenv("CONSUL_HTTP_TOKEN_FILE", fullname)

		require.NoError(t, c.flags.Parse(args))

		tok, err := c.loadToken()
		require.NoError(t, err)
		require.Equal(t, testToken, tok)
	})
}

func TestWatchCommandNoConnect(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-type=connect_leaf"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.ErrorWriter.String(),
		"Type connect_leaf is not supported in the CLI tool") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}

func TestWatchCommandNoAgentService(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-type=agent_service"}

	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.ErrorWriter.String(),
		"Type agent_service is not supported in the CLI tool") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
