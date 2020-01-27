package read

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestConfigRead_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConfigRead(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	_, _, err := client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "web",
		Protocol: "tcp",
	}, nil)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-kind=" + api.ServiceDefaults,
		"-name=web",
	}

	code := c.Run(args)
	require.Equal(t, 0, code)

	entry, err := api.DecodeConfigEntryFromJSON(ui.OutputWriter.Bytes())
	require.NoError(t, err)
	svc, ok := entry.(*api.ServiceConfigEntry)
	require.True(t, ok)
	require.Equal(t, api.ServiceDefaults, svc.Kind)
	require.Equal(t, "web", svc.Name)
	require.Equal(t, "tcp", svc.Protocol)
}

func TestConfigRead_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no kind": []string{},
		"no name": []string{"-kind", "service-defaults"},
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
