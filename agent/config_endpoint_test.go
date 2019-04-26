package agent

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/testrpc"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestConfig_Get(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some config entries.
	reqs := []structs.ConfigEntryRequest{
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name: "foo",
			},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name: "bar",
			},
		},
	}
	for _, req := range reqs {
		var out struct{}
		require.NoError(a.RPC("ConfigEntry.Apply", &req, &out))
	}

	t.Run("get a single service entry", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/service-defaults/foo", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.Config(resp, req)
		require.NoError(err)

		value := obj.(structs.IndexedConfigEntries)
		require.Equal(structs.ServiceDefaults, value.Kind)
		require.Len(value.Entries, 1)
		entry := value.Entries[0].(*structs.ServiceConfigEntry)
		require.Equal(entry.Name, "foo")
	})
	t.Run("list both service entries", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/service-defaults", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.Config(resp, req)
		require.NoError(err)

		value := obj.(structs.IndexedConfigEntries)
		require.Equal(structs.ServiceDefaults, value.Kind)
		require.Len(value.Entries, 2)
		require.Equal(value.Entries[0].(*structs.ServiceConfigEntry).Name, "bar")
		require.Equal(value.Entries[1].(*structs.ServiceConfigEntry).Name, "foo")
	})
	t.Run("error on no arguments", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/", nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.Config(resp, req)
		require.Error(errors.New("Must provide either a kind or both kind and name"), err)
	})
}

func TestConfig_Delete(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some config entries.
	reqs := []structs.ConfigEntryRequest{
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name: "foo",
			},
		},
		{
			Datacenter: "dc1",
			Entry: &structs.ServiceConfigEntry{
				Name: "bar",
			},
		},
	}
	for _, req := range reqs {
		var out struct{}
		require.NoError(a.RPC("ConfigEntry.Apply", &req, &out))
	}

	// Delete an entry.
	{
		req, _ := http.NewRequest("DELETE", "/v1/config/service-defaults/bar", nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.Config(resp, req)
		require.NoError(err)
	}
	// Get the remaining entry.
	{
		args := structs.ConfigEntryQuery{
			Kind:       structs.ServiceDefaults,
			Datacenter: "dc1",
		}
		var out structs.IndexedConfigEntries
		require.NoError(a.RPC("ConfigEntry.List", &args, &out))
		require.Equal(structs.ServiceDefaults, out.Kind)
		require.Len(out.Entries, 1)
		entry := out.Entries[0].(*structs.ServiceConfigEntry)
		require.Equal(entry.Name, "foo")
	}
}

func TestConfig_Apply(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some config entries.
	body := bytes.NewBuffer([]byte(`
	{
		"Kind": "service-defaults",
		"Name": "foo",
		"Protocol": "tcp"
	}`))

	req, _ := http.NewRequest("PUT", "/v1/config", body)
	resp := httptest.NewRecorder()
	_, err := a.srv.ConfigApply(resp, req)
	require.NoError(err)
	if resp.Code != 200 {
		t.Fatalf(resp.Body.String())
	}

	// Get the remaining entry.
	{
		args := structs.ConfigEntryQuery{
			Kind:       structs.ServiceDefaults,
			Name:       "foo",
			Datacenter: "dc1",
		}
		var out structs.IndexedConfigEntries
		require.NoError(a.RPC("ConfigEntry.Get", &args, &out))
		require.Equal(structs.ServiceDefaults, out.Kind)
		require.Len(out.Entries, 1)
		entry := out.Entries[0].(*structs.ServiceConfigEntry)
		require.Equal(entry.Name, "foo")
	}
}
