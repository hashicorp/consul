package expose

import (
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func TestConnectExpose(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	client := a.Client()
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=foo",
			"-ingress-gateway=ingress",
			"-port=8888",
			"-protocol=tcp",
		}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
	}

	// Make sure the config entry and intention have been created.
	entry, _, err := client.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
	require.NoError(t, err)
	ns := entry.(*api.IngressGatewayConfigEntry).Namespace
	ap := entry.(*api.IngressGatewayConfigEntry).Partition
	expected := &api.IngressGatewayConfigEntry{
		Kind:      api.IngressGateway,
		Name:      "ingress",
		Namespace: ns,
		Partition: ap,
		Listeners: []api.IngressListener{
			{
				Port:     8888,
				Protocol: "tcp",
				Services: []api.IngressService{
					{
						Name:      "foo",
						Namespace: ns,
						Partition: ap,
					},
				},
			},
		},
	}
	expected.CreateIndex = entry.GetCreateIndex()
	expected.ModifyIndex = entry.GetModifyIndex()
	require.Equal(t, expected, entry)

	ixns, _, err := client.Connect().Intentions(nil)
	require.NoError(t, err)
	require.Len(t, ixns, 1)
	require.Equal(t, "ingress", ixns[0].SourceName)
	require.Equal(t, "foo", ixns[0].DestinationName)

	// Run the command again with a different port, make sure the config entry
	// is updated while intentions are unmodified.
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=foo",
			"-ingress-gateway=ingress",
			"-port=7777",
			"-protocol=tcp",
		}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		expected.Listeners = append(expected.Listeners, api.IngressListener{
			Port:     7777,
			Protocol: "tcp",
			Services: []api.IngressService{
				{
					Name:      "foo",
					Namespace: ns,
					Partition: ap,
				},
			},
		})

		// Make sure the config entry/intention weren't affected.
		entry, _, err = client.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
		require.NoError(t, err)
		expected.ModifyIndex = entry.GetModifyIndex()
		require.Equal(t, expected, entry)

		ixns, _, err = client.Connect().Intentions(nil)
		require.NoError(t, err)
		require.Len(t, ixns, 1)
		require.Equal(t, "ingress", ixns[0].SourceName)
		require.Equal(t, "foo", ixns[0].DestinationName)
	}

	// Run the command again with a conflicting protocol, should exit with an error and
	// cause no changes to config entry/intentions.
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=bar",
			"-ingress-gateway=ingress",
			"-port=8888",
			"-protocol=http",
		}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		require.Contains(t, ui.ErrorWriter.String(), `conflicting protocol "tcp"`)

		// Make sure the config entry/intention weren't affected.
		entry, _, err = client.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
		require.NoError(t, err)
		require.Equal(t, expected, entry)

		ixns, _, err = client.Connect().Intentions(nil)
		require.NoError(t, err)
		require.Len(t, ixns, 1)
		require.Equal(t, "ingress", ixns[0].SourceName)
		require.Equal(t, "foo", ixns[0].DestinationName)
	}
}

func TestConnectExpose_invalidFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	t.Run("missing service", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		require.Contains(t, ui.ErrorWriter.String(), "A service name must be given")
	})
	t.Run("missing gateway", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=foo",
		}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		require.Contains(t, ui.ErrorWriter.String(), "An ingress gateway service must be given")
	})
	t.Run("missing port", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=foo",
			"-ingress-gateway=ingress",
		}

		code := c.Run(args)
		if code != 1 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}
		require.Contains(t, ui.ErrorWriter.String(), "A port must be provided")
	})
}

func TestConnectExpose_existingConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	client := a.Client()
	defer a.Shutdown()

	// Create some service config entries to set their protocol.
	for _, service := range []string{"bar", "zoo"} {
		_, _, err := client.ConfigEntries().Set(&api.ServiceConfigEntry{
			Kind:     "service-defaults",
			Name:     service,
			Protocol: "http",
		}, nil)
		require.NoError(t, err)
	}

	// Create an existing ingress config entry with some services.
	ingressConf := &api.IngressGatewayConfigEntry{
		Kind: api.IngressGateway,
		Name: "ingress",
		Listeners: []api.IngressListener{
			{
				Port:     8888,
				Protocol: "tcp",
				Services: []api.IngressService{
					{
						Name: "foo",
					},
				},
			},
			{
				Port:     9999,
				Protocol: "http",
				Services: []api.IngressService{
					{
						Name: "bar",
					},
				},
			},
		},
	}
	_, _, err := client.ConfigEntries().Set(ingressConf, nil)
	require.NoError(t, err)

	// Add a service on a new port.
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=baz",
			"-ingress-gateway=ingress",
			"-port=10000",
			"-protocol=tcp",
		}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the ingress config was updated and existing services preserved.
		entry, _, err := client.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
		require.NoError(t, err)

		entryConf := entry.(*api.IngressGatewayConfigEntry)
		ingressConf.Listeners = append(ingressConf.Listeners, api.IngressListener{
			Port:     10000,
			Protocol: "tcp",
			Services: []api.IngressService{
				{
					Name: "baz",
				},
			},
		})
		ingressConf.Partition = entryConf.Partition
		ingressConf.Namespace = entryConf.Namespace
		for i, listener := range ingressConf.Listeners {
			listener.Services[0].Namespace = entryConf.Listeners[i].Services[0].Namespace
			listener.Services[0].Partition = entryConf.Listeners[i].Services[0].Partition
		}
		ingressConf.CreateIndex = entry.GetCreateIndex()
		ingressConf.ModifyIndex = entry.GetModifyIndex()
		require.Equal(t, ingressConf, entry)
	}

	// Add an service on a port shared with an existing listener.
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=zoo",
			"-ingress-gateway=ingress",
			"-port=9999",
			"-protocol=http",
			"-host=foo.com",
			"-host=foo.net",
		}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the ingress config was updated and existing services preserved.
		entry, _, err := client.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
		require.NoError(t, err)

		entryConf := entry.(*api.IngressGatewayConfigEntry)
		ingressConf.Listeners[1].Services = append(ingressConf.Listeners[1].Services, api.IngressService{
			Name:      "zoo",
			Namespace: entryConf.Listeners[1].Services[1].Namespace,
			Partition: entryConf.Listeners[1].Services[1].Partition,
			Hosts:     []string{"foo.com", "foo.net"},
		})
		ingressConf.CreateIndex = entry.GetCreateIndex()
		ingressConf.ModifyIndex = entry.GetModifyIndex()
		require.Equal(t, ingressConf, entry)
	}

	// Update the bar service and add a custom host.
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	{
		ui := cli.NewMockUi()
		c := New(ui)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service=bar",
			"-ingress-gateway=ingress",
			"-port=9999",
			"-protocol=http",
			"-host=bar.com",
		}

		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
		}

		// Make sure the ingress config was updated and existing services preserved.
		entry, _, err := client.ConfigEntries().Get(api.IngressGateway, "ingress", nil)
		require.NoError(t, err)

		ingressConf.Listeners[1].Services[0].Hosts = []string{"bar.com"}
		ingressConf.CreateIndex = entry.GetCreateIndex()
		ingressConf.ModifyIndex = entry.GetModifyIndex()
		require.Equal(t, ingressConf, entry)
	}
}
