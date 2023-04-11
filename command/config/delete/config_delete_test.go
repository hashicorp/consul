package delete

import (
	"strconv"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
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

	err := createEntry(client)
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

func createEntry(client *api.Client) error {
	_, _, err := client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "web",
		Protocol: "tcp",
	}, nil)
	return err
}

func TestConfigDelete_CAS(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	err := createEntry(client)
	require.NoError(t, err)

	entry, _, err := client.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
	require.NoError(t, err)

	t.Run("with an invalid modify index", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-kind=" + api.ServiceDefaults,
			"-name=web",
			"-cas",
			"-modify-index=" + strconv.FormatUint(entry.GetModifyIndex()-1, 10),
		}

		code := c.Run(args)
		require.Equal(t, 1, code)
		require.Contains(t, ui.ErrorWriter.String(),
			"Config entry not deleted: service-defaults/web")
		require.Empty(t, ui.OutputWriter.String())
	})

	t.Run("with a valid modify index", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-kind=" + api.ServiceDefaults,
			"-name=web",
			"-cas",
			"-modify-index=" + strconv.FormatUint(entry.GetModifyIndex(), 10),
		}

		code := c.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(),
			"Config entry deleted: service-defaults/web")
		require.Empty(t, ui.ErrorWriter.String())
		entry, _, err := client.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
		require.Error(t, err)
		require.Nil(t, entry)
	})

	t.Run("delete from file with a valid modify index", func(t *testing.T) {
		err := createEntry(client)
		require.NoError(t, err)
		entry, _, err := client.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
		require.NoError(t, err)

		ui := cli.NewMockUi()
		c := New(ui)
		f := testutil.TempFile(t, "config-write-svc-web.hcl")
		_, err = f.WriteString(`
	  Kind = "service-defaults"
	  Name = "web"
	  Protocol = "tcp"
	  `)
		require.NoError(t, err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-filename=" + f.Name(),
			"-cas",
			"-modify-index=" + strconv.FormatUint(entry.GetModifyIndex(), 10),
		}

		code := c.Run(args)
		require.Equal(t, 0, code)
		require.Contains(t, ui.OutputWriter.String(),
			"Config entry deleted: service-defaults/web")
		require.Empty(t, ui.ErrorWriter.String())

		entry, _, err = client.ConfigEntries().Get(api.ServiceDefaults, "web", nil)
		require.Error(t, err)
		require.Nil(t, entry)
	})
}

func TestConfigDelete_InvalidArgs(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		args []string
		err  string
	}{
		"no kind or filename": {
			args: []string{},
			err:  "Must specify the -kind or -filename parameter",
		},
		"no kind": {
			args: []string{"-name", "web"},
			err:  "Must specify the -kind parameter",
		},
		"no name": {
			args: []string{"-kind", api.ServiceDefaults},
			err:  "Must specify the -name parameter",
		},
		"cas but no modify-index": {
			args: []string{"-kind", api.ServiceDefaults, "-name", "web", "-cas"},
			err:  "Must specify a -modify-index greater than 0 with -cas",
		},
		"cas but no zero modify-index": {
			args: []string{"-kind", api.ServiceDefaults, "-name", "web", "-cas", "-modify-index", "0"},
			err:  "Must specify a -modify-index greater than 0 with -cas",
		},
		"modify-index but no cas": {
			args: []string{"-kind", api.ServiceDefaults, "-name", "web", "-modify-index", "1"},
			err:  "Cannot specify -modify-index without -cas",
		},
		"kind and filename": {
			args: []string{"-kind", api.ServiceDefaults, "-filename", "config-file.hcl"},
			err:  "filename can't be used with kind or name",
		},
		"name and filename": {
			args: []string{"-name", "db", "-filename", "config-file.hcl"},
			err:  "filename can't be used with kind or name",
		},
		"kind, name, and filename": {
			args: []string{"-kind", api.ServiceDefaults, "-name", "db", "-filename", "config-file.hcl"},
			err:  "filename can't be used with kind or name",
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			ui := cli.NewMockUi()
			c := New(ui)

			require.NotEqual(t, 0, c.Run(tcase.args))
			require.Contains(t, ui.ErrorWriter.String(), tcase.err)
		})
	}
}
