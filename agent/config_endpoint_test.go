package agent

import (
	"bytes"
	"fmt"
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
		{
			Datacenter: "dc1",
			Entry: &structs.ProxyConfigEntry{
				Name: structs.ProxyConfigGlobal,
				Config: map[string]interface{}{
					"foo": "bar",
					"bar": 1,
				},
			},
		},
	}
	for _, req := range reqs {
		out := false
		require.NoError(t, a.RPC("ConfigEntry.Apply", &req, &out))
	}

	t.Run("get a single service entry", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/service-defaults/foo", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.Config(resp, req)
		require.NoError(t, err)

		value := obj.(structs.ConfigEntry)
		require.Equal(t, structs.ServiceDefaults, value.GetKind())
		entry := value.(*structs.ServiceConfigEntry)
		require.Equal(t, entry.Name, "foo")
	})
	t.Run("list both service entries", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/service-defaults", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.Config(resp, req)
		require.NoError(t, err)

		value := obj.([]structs.ConfigEntry)
		require.Len(t, value, 2)
		require.Equal(t, value[0].(*structs.ServiceConfigEntry).Name, "bar")
		require.Equal(t, value[1].(*structs.ServiceConfigEntry).Name, "foo")
	})
	t.Run("get global proxy config", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/proxy-defaults/global", nil)
		resp := httptest.NewRecorder()
		obj, err := a.srv.Config(resp, req)
		require.NoError(t, err)

		value := obj.(structs.ConfigEntry)
		require.Equal(t, value.GetKind(), structs.ProxyDefaults)
		entry := value.(*structs.ProxyConfigEntry)
		require.Equal(t, structs.ProxyConfigGlobal, entry.Name)
		require.Contains(t, entry.Config, "foo")
		require.Equal(t, "bar", entry.Config["foo"])
	})
	t.Run("error on no arguments", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/v1/config/", nil)
		resp := httptest.NewRecorder()
		_, err := a.srv.Config(resp, req)
		require.Error(t, errors.New("Must provide either a kind or both kind and name"), err)
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
		out := false
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
		var out structs.ConfigEntryResponse
		require.NoError(a.RPC("ConfigEntry.Get", &args, &out))
		require.NotNil(out.Entry)
		entry := out.Entry.(*structs.ServiceConfigEntry)
		require.Equal(entry.Name, "foo")
	}
}

func TestConfig_Apply_ProxyDefaultsMeshGateway(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some config entries.
	body := bytes.NewBuffer([]byte(`
	{
		"Kind": "proxy-defaults",
		"Name": "global",
		"MeshGateway": {
			"Mode": "local"
		}
	}`))

	req, _ := http.NewRequest("PUT", "/v1/config", body)
	resp := httptest.NewRecorder()
	_, err := a.srv.ConfigApply(resp, req)
	require.NoError(t, err)
	require.Equal(t, 200, resp.Code, "!200 Response Code: %s", resp.Body.String())

	// Get the remaining entry.
	{
		args := structs.ConfigEntryQuery{
			Kind:       structs.ProxyDefaults,
			Name:       "global",
			Datacenter: "dc1",
		}
		var out structs.ConfigEntryResponse
		require.NoError(t, a.RPC("ConfigEntry.Get", &args, &out))
		require.NotNil(t, out.Entry)
		entry := out.Entry.(*structs.ProxyConfigEntry)
		require.Equal(t, structs.MeshGatewayModeLocal, entry.MeshGateway.Mode)
	}
}

func TestConfig_Apply_CAS(t *testing.T) {
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

	// Get the entry remaining entry.
	args := structs.ConfigEntryQuery{
		Kind:       structs.ServiceDefaults,
		Name:       "foo",
		Datacenter: "dc1",
	}

	out := &structs.ConfigEntryResponse{}
	require.NoError(a.RPC("ConfigEntry.Get", &args, out))
	require.NotNil(out.Entry)
	entry := out.Entry.(*structs.ServiceConfigEntry)

	body = bytes.NewBuffer([]byte(`
	{
		"Kind": "service-defaults",
		"Name": "foo",
		"Protocol": "udp"
	}
	`))
	req, _ = http.NewRequest("PUT", "/v1/config?cas=0", body)
	resp = httptest.NewRecorder()
	writtenRaw, err := a.srv.ConfigApply(resp, req)
	require.NoError(err)
	written, ok := writtenRaw.(bool)
	require.True(ok)
	require.False(written)
	require.EqualValues(200, resp.Code, resp.Body.String())

	body = bytes.NewBuffer([]byte(`
	{
		"Kind": "service-defaults",
		"Name": "foo",
		"Protocol": "udp"
	}
	`))
	req, _ = http.NewRequest("PUT", fmt.Sprintf("/v1/config?cas=%d", entry.GetRaftIndex().ModifyIndex), body)
	resp = httptest.NewRecorder()
	writtenRaw, err = a.srv.ConfigApply(resp, req)
	require.NoError(err)
	written, ok = writtenRaw.(bool)
	require.True(ok)
	require.True(written)
	require.EqualValues(200, resp.Code, resp.Body.String())

	// Get the entry remaining entry.
	args = structs.ConfigEntryQuery{
		Kind:       structs.ServiceDefaults,
		Name:       "foo",
		Datacenter: "dc1",
	}

	out = &structs.ConfigEntryResponse{}
	require.NoError(a.RPC("ConfigEntry.Get", &args, out))
	require.NotNil(out.Entry)
	newEntry := out.Entry.(*structs.ServiceConfigEntry)
	require.NotEqual(entry.GetRaftIndex(), newEntry.GetRaftIndex())
}

func TestConfig_Apply_Decoding(t *testing.T) {
	t.Parallel()

	a := NewTestAgent(t, t.Name(), "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	t.Run("No Kind", func(t *testing.T) {
		body := bytes.NewBuffer([]byte(
			`{
			"Name": "foo",
			"Protocol": "tcp"
		}`))

		req, _ := http.NewRequest("PUT", "/v1/config", body)
		resp := httptest.NewRecorder()

		_, err := a.srv.ConfigApply(resp, req)
		require.Error(t, err)
		badReq, ok := err.(BadRequestError)
		require.True(t, ok)
		require.Equal(t, "Request decoding failed: Payload does not contain a kind/Kind key at the top level", badReq.Reason)
	})

	t.Run("Kind Not String", func(t *testing.T) {
		body := bytes.NewBuffer([]byte(
			`{
			"Kind": 123,
			"Name": "foo",
			"Protocol": "tcp"
		}`))

		req, _ := http.NewRequest("PUT", "/v1/config", body)
		resp := httptest.NewRecorder()

		_, err := a.srv.ConfigApply(resp, req)
		require.Error(t, err)
		badReq, ok := err.(BadRequestError)
		require.True(t, ok)
		require.Equal(t, "Request decoding failed: Kind value in payload is not a string", badReq.Reason)
	})

	t.Run("Lowercase kind", func(t *testing.T) {
		body := bytes.NewBuffer([]byte(
			`{
			"kind": "service-defaults",
			"Name": "foo",
			"Protocol": "tcp"
		}`))

		req, _ := http.NewRequest("PUT", "/v1/config", body)
		resp := httptest.NewRecorder()
		_, err := a.srv.ConfigApply(resp, req)
		require.NoError(t, err)
		require.EqualValues(t, 200, resp.Code, resp.Body.String())

		// Get the remaining entry.
		{
			args := structs.ConfigEntryQuery{
				Kind:       structs.ServiceDefaults,
				Name:       "foo",
				Datacenter: "dc1",
			}
			var out structs.ConfigEntryResponse
			require.NoError(t, a.RPC("ConfigEntry.Get", &args, &out))
			require.NotNil(t, out.Entry)
			entry := out.Entry.(*structs.ServiceConfigEntry)
			require.Equal(t, entry.Name, "foo")
		}
	})
}
