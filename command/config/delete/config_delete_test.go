package delete

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestConfigDelete_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConfigDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	_, _, err := client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "web",
		Protocol: "tcp",
	}, nil)
	require.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-kind=" + api.ServiceDefaults,
		"-name=web",
	}

	code := c.Run(args)
	require.Equal(t, 0, code)
	require.Contains(t, ui.OutputWriter.String(),
		"Config entry deleted: service-defaults/web")
	require.Empty(t, ui.ErrorWriter.String())

	entry, _, err := client.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
	require.Error(t, err)
	require.Nil(t, entry)
}

func TestConfigDelete_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no kind": {},
		"no name": {"-kind", "service-defaults"},
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
