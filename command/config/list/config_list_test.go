package list

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestConfigList_noTabs(t *testing.T) {
	t.Parallel()

	require.NotContains(t, New(cli.NewMockUi()).Help(), "\t")
}

func TestConfigList(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

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

	_, _, err = client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "foo",
		Protocol: "tcp",
	}, nil)
	require.NoError(t, err)

	_, _, err = client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "api",
		Protocol: "tcp",
	}, nil)
	require.NoError(t, err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-kind=" + api.ServiceDefaults,
	}

	code := c.Run(args)
	require.Equal(t, 0, code)

	services := strings.Split(strings.Trim(ui.OutputWriter.String(), "\n"), "\n")

	require.ElementsMatch(t, []string{"web", "foo", "api"}, services)
}

func TestConfigList_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"no kind": {},
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
