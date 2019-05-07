package write

import (
	"io"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestConfigWrite_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConfigWrite(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	t.Run("File", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		f := testutil.TempFile(t, "config-write-svc-web.hcl")
		defer os.Remove(f.Name())
		_, err := f.WriteString(`
      Kind = "service-defaults"
      Name = "web"
      Protocol = "udp"
      `)

		require.NoError(t, err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			f.Name(),
		}

		code := c.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get("service-defaults", "web", nil)
		require.NoError(t, err)
		svc, ok := entry.(*api.ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, api.ServiceDefaults, svc.Kind)
		require.Equal(t, "web", svc.Name)
		require.Equal(t, "udp", svc.Protocol)
	})

	t.Run("Stdin", func(t *testing.T) {
		stdinR, stdinW := io.Pipe()

		ui := cli.NewMockUi()
		c := New(ui)
		c.testStdin = stdinR

		go func() {
			stdinW.Write([]byte(`{
            "Kind": "proxy-defaults",
            "Name": "global",
            "Config": {
               "foo": "bar",
               "bar": 1.0
            }
         }`))
			stdinW.Close()
		}()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-",
		}

		code := c.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, 0, code)

		entry, _, err := client.ConfigEntries().Get(api.ProxyDefaults, api.ProxyConfigGlobal, nil)
		require.NoError(t, err)
		proxy, ok := entry.(*api.ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, api.ProxyDefaults, proxy.Kind)
		require.Equal(t, api.ProxyConfigGlobal, proxy.Name)
		require.Equal(t, map[string]interface{}{"foo": "bar", "bar": 1.0}, proxy.Config)
	})

	t.Run("No config", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		code := c.Run([]string{})
		require.NotEqual(t, 0, code)
		require.NotEmpty(t, ui.ErrorWriter.String())
	})

}
