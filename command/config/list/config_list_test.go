// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package list

import (
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
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
		Protocol: "http",
	}, nil)
	require.NoError(t, err)

	cases := map[string]struct {
		args     []string
		expected []string
		errMsg   string
	}{
		"list service-defaults": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-kind=" + api.ServiceDefaults,
			},
			expected: []string{"web", "foo", "api"},
		},
		"filter service-defaults": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-kind=" + api.ServiceDefaults,
				"-filter=" + `Protocol == "http"`,
			},
			expected: []string{"api"},
		},
		"filter unsupported kind": {
			args: []string{
				"-http-addr=" + a.HTTPAddr(),
				"-kind=" + api.ProxyDefaults,
				"-filter", `Mode == "transparent"`,
			},
			errMsg: "filtering not supported for config entry kind=proxy-defaults",
		},
	}
	for name, c := range cases {
		c := c
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			cmd := New(ui)

			code := cmd.Run(c.args)

			if c.errMsg == "" {
				require.Equal(t, 0, code)
				services := strings.Split(strings.Trim(ui.OutputWriter.String(), "\n"), "\n")
				require.ElementsMatch(t, c.expected, services)
			} else {
				require.NotEqual(t, 0, code)
				require.Contains(t, ui.ErrorWriter.String(), c.errMsg)
			}

		})
	}
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
