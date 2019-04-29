package enable

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestConnectEnable_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConnectEnable(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-service=web",
		"-protocol=tcp",
		"-sidecar-proxy=true",
	}

	code := c.Run(args)
	require.Equal(t, 0, code)

	entry, _, err := client.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
	require.NoError(t, err)
	svc, ok := entry.(*api.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, api.ServiceDefaults, svc.Kind)
	require.Equal(t, "web", svc.Name)
	require.Equal(t, "tcp", svc.Protocol)
	require.True(t, svc.Connect.SidecarProxy)
}

func TestConnectEnable_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no service": []string{},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			require.NotEqual(t, 0, c.Run(tcase))
			require.NotEmpty(t, ui.ErrorWriter.String())
		})
	}
}
