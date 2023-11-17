package generate

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestGenerateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestGenerateCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	t.Cleanup(func() { _ = a.Shutdown() })
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	client := a.Client()

	t.Run("name is required", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "err: %s", ui.ErrorWriter.String())
		require.Contains(t, ui.ErrorWriter.String(), "Missing the required -name flag")
	})

	t.Run("invalid format", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=foo",
			"-format=toml",
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, "exited successfully when it should have failed")
		output := ui.ErrorWriter.String()
		require.Contains(t, output, "Invalid format")
	})

	t.Run("generate token", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=foo",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		token, err := base64.StdEncoding.DecodeString(ui.OutputWriter.String())
		require.NoError(t, err, "error decoding token")
		require.Contains(t, string(token), "\"ServerName\":\"server.dc1.peering.11111111-2222-3333-4444-555555555555.consul\"")
	})

	t.Run("generate token with options", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=bar",
			"-server-external-addresses=1.2.3.4,5.6.7.8",
			"-meta=env=production",
			"-meta=region=us-east-1",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		token, err := base64.StdEncoding.DecodeString(ui.OutputWriter.String())
		require.NoError(t, err, "error decoding token")
		require.Contains(t, string(token), "\"ServerName\":\"server.dc1.peering.11111111-2222-3333-4444-555555555555.consul\"")

		// ServerExternalAddresses
		require.Contains(t, string(token), "1.2.3.4")
		require.Contains(t, string(token), "5.6.7.8")

		// Meta
		peering, _, err := client.Peerings().Read(context.Background(), "bar", &api.QueryOptions{})
		require.NoError(t, err)

		actual, ok := peering.Meta["env"]
		require.True(t, ok)
		require.Equal(t, "production", actual)

		actual, ok = peering.Meta["region"]
		require.True(t, ok)
		require.Equal(t, "us-east-1", actual)
	})

	t.Run("read with json", func(t *testing.T) {

		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-name=baz",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Equal(t, 0, code)
		output := ui.OutputWriter.Bytes()

		var outputRes api.PeeringGenerateTokenResponse
		require.NoError(t, json.Unmarshal(output, &outputRes))

		token, err := base64.StdEncoding.DecodeString(outputRes.PeeringToken)
		require.NoError(t, err, "error decoding token")
		require.Contains(t, string(token), "\"ServerName\":\"server.dc1.peering.11111111-2222-3333-4444-555555555555.consul\"")
	})
}
