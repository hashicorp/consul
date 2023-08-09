package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntries_HTTPRoute(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	configEntries := c.ConfigEntries()
	route1 := &HTTPRouteConfigEntry{
		Kind:        HTTPRoute,
		Name:        "route1",
		Parents:     []ResourceReference{},
		Rules:       []HTTPRouteRule{},
		Hostnames:   []string{},
		Meta:        map[string]string{},
		CreateIndex: 0,
		ModifyIndex: 0,
		Partition:   "",
		Namespace:   "",
		Status:      ConfigEntryStatus{},
	}

	route2 := &HTTPRouteConfigEntry{
		Kind:        HTTPRoute,
		Name:        "route2",
		Parents:     []ResourceReference{},
		Rules:       []HTTPRouteRule{},
		Hostnames:   []string{},
		Meta:        map[string]string{},
		CreateIndex: 0,
		ModifyIndex: 0,
		Partition:   "",
		Namespace:   "",
		Status:      ConfigEntryStatus{},
	}

	// set it
	_, wm, err := configEntries.Set(route1, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// also set the second one
	_, wm, err = configEntries.Set(route2, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// get it
	entry, qm, err := configEntries.Get(HTTPRoute, "route1", nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)

	// verify it
	readRoute, ok := entry.(*HTTPRouteConfigEntry)
	require.True(t, ok)
	require.Equal(t, route1.Kind, readRoute.Kind)
	require.Equal(t, route1.Name, readRoute.Name)
	require.Equal(t, route1.Meta, readRoute.Meta)
	require.Equal(t, route1.Meta, readRoute.GetMeta())

	// update it
	route1.Rules = []HTTPRouteRule{
		{
			Filters: HTTPFilters{
				URLRewrite: &URLRewrite{
					Path: "abc",
				},
			},
		},
	}

	// CAS fail
	written, _, err := configEntries.CAS(route1, 0, nil)
	require.NoError(t, err)
	require.False(t, written)

	// CAS success
	written, wm, err = configEntries.CAS(route1, readRoute.ModifyIndex, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)
	require.True(t, written)

	// re-setting should not yield an error
	_, wm, err = configEntries.Set(route1, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	route2.Rules = []HTTPRouteRule{
		{
			Filters: HTTPFilters{
				URLRewrite: &URLRewrite{
					Path: "def",
				},
			},
		},
	}

	_, wm, err = configEntries.Set(route2, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// list them
	entries, qm, err := configEntries.List(HTTPRoute, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)
	require.Len(t, entries, 2)

	for _, entry = range entries {
		switch entry.GetName() {
		case "route1":
			// this also verifies that the update value was persisted and
			// the updated values are seen
			readRoute, ok = entry.(*HTTPRouteConfigEntry)
			require.True(t, ok)
			require.Equal(t, route1.Kind, readRoute.Kind)
			require.Equal(t, route1.Name, readRoute.Name)
			require.Len(t, readRoute.Rules, 1)

			require.Equal(t, route1.Rules, readRoute.Rules)
		case "route2":
			readRoute, ok = entry.(*HTTPRouteConfigEntry)
			require.True(t, ok)
			require.Equal(t, route2.Kind, readRoute.Kind)
			require.Equal(t, route2.Name, readRoute.Name)
			require.Len(t, readRoute.Rules, 1)

			require.Equal(t, route2.Rules, readRoute.Rules)
		}
	}

	// delete it
	wm, err = configEntries.Delete(HTTPRoute, "route1", nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// verify deletion
	_, _, err = configEntries.Get(HTTPRoute, "route1", nil)
	require.Error(t, err)
}
