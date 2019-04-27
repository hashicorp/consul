package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntry(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	config := c.ConfigEntries()

	t.Run("Proxy Defaults", func(t *testing.T) {
		global_proxy := &ProxyConfigEntry{
			Kind: ProxyDefaults,
			Name: ProxyConfigGlobal,
			Config: map[string]interface{}{
				"foo": "bar",
				"bar": 1.0,
			},
		}

		// set it
		wm, err := config.ConfigEntrySet(global_proxy, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config.ConfigEntryGet(ProxyDefaults, ProxyConfigGlobal, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readProxy, ok := entry.(*ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, global_proxy.Kind, readProxy.Kind)
		require.Equal(t, global_proxy.Name, readProxy.Name)
		require.Equal(t, global_proxy.Config, readProxy.Config)

		// update it
		global_proxy.Config["baz"] = true
		wm, err = config.ConfigEntrySet(global_proxy, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// list it
		entries, qm, err := config.ConfigEntryList(ProxyDefaults, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)
		require.Len(t, entries, 1)
		readProxy, ok = entries[0].(*ProxyConfigEntry)
		require.True(t, ok)
		require.Equal(t, global_proxy.Kind, readProxy.Kind)
		require.Equal(t, global_proxy.Name, readProxy.Name)
		require.Equal(t, global_proxy.Config, readProxy.Config)

		// delete it
		wm, err = config.ConfigEntryDelete(ProxyDefaults, ProxyConfigGlobal, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		entry, qm, err = config.ConfigEntryGet(ProxyDefaults, ProxyConfigGlobal, nil)
		require.Error(t, err)
	})

	t.Run("Service Defaults", func(t *testing.T) {
		service := &ServiceConfigEntry{
			Kind:     ServiceDefaults,
			Name:     "foo",
			Protocol: "udp",
		}

		service2 := &ServiceConfigEntry{
			Kind:     ServiceDefaults,
			Name:     "bar",
			Protocol: "tcp",
		}

		// set it
		wm, err := config.ConfigEntrySet(service, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// also set the second one
		wm, err = config.ConfigEntrySet(service2, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// get it
		entry, qm, err := config.ConfigEntryGet(ServiceDefaults, "foo", nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)

		// verify it
		readService, ok := entry.(*ServiceConfigEntry)
		require.True(t, ok)
		require.Equal(t, service.Kind, readService.Kind)
		require.Equal(t, service.Name, readService.Name)
		require.Equal(t, service.Protocol, readService.Protocol)

		// update it
		service.Protocol = "tcp"
		wm, err = config.ConfigEntrySet(service, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// list them
		entries, qm, err := config.ConfigEntryList(ServiceDefaults, nil)
		require.NoError(t, err)
		require.NotNil(t, qm)
		require.NotEqual(t, 0, qm.RequestTime)
		require.Len(t, entries, 2)

		for _, entry = range entries {
			if entry.GetName() == "foo" {
				readService, ok = entry.(*ServiceConfigEntry)
				require.True(t, ok)
				require.Equal(t, service.Kind, readService.Kind)
				require.Equal(t, service.Name, readService.Name)
				require.Equal(t, service.Protocol, readService.Protocol)
			}
		}

		// delete it
		wm, err = config.ConfigEntryDelete(ServiceDefaults, "foo", nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)

		// verify deletion
		entry, qm, err = config.ConfigEntryGet(ServiceDefaults, "foo", nil)
		require.Error(t, err)
	})
}
