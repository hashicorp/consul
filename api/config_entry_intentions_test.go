package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntries_ServiceIntentions(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForServiceIntentions(t)

	config_entries := c.ConfigEntries()

	// Allow L7 for all services.
	_, _, err := config_entries.Set(&ProxyConfigEntry{
		Kind: ProxyDefaults,
		Name: ProxyConfigGlobal,
		Config: map[string]interface{}{
			"protocol": "http",
		},
	}, nil)
	require.NoError(t, err)

	entries := []*ServiceIntentionsConfigEntry{
		{
			Kind: ServiceIntentions,
			Name: "foo",
			Sources: []*SourceIntention{
				{
					Name:   "one",
					Action: IntentionActionAllow,
				},
				{
					Name:   "two",
					Action: IntentionActionDeny,
				},
			},
		},
		{
			Kind: ServiceIntentions,
			Name: "bar",
			Sources: []*SourceIntention{
				{
					Name:   "three",
					Action: IntentionActionAllow,
				},
			},
		},
	}

	// set them
	for _, entry := range entries {
		_, wm, err := config_entries.Set(entry, nil)
		require.NoError(t, err)
		require.NotNil(t, wm)
		require.NotEqual(t, 0, wm.RequestTime)
	}

	// get one
	entry, qm, err := config_entries.Get(ServiceIntentions, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)

	// verify it
	readIxn, ok := entry.(*ServiceIntentionsConfigEntry)
	require.True(t, ok)
	require.Equal(t, "service-intentions", readIxn.Kind)
	require.Equal(t, "foo", readIxn.Name)
	require.Len(t, readIxn.Sources, 2)

	// update it
	entries[0].Meta = map[string]string{"a": "b"}

	// CAS fail
	written, _, err := config_entries.CAS(entries[0], 0, nil)
	require.NoError(t, err)
	require.False(t, written)

	// CAS success
	written, wm, err := config_entries.CAS(entries[0], readIxn.ModifyIndex, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)
	require.True(t, written)

	// update no cas
	entries[0].Meta = map[string]string{"x": "y"}

	_, wm, err = config_entries.Set(entries[0], nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// list them
	gotEntries, qm, err := config_entries.List(ServiceIntentions, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	require.NotEqual(t, 0, qm.RequestTime)
	require.Len(t, gotEntries, 2)

	for _, entry = range gotEntries {
		switch entry.GetName() {
		case "foo":
			// this also verifies that the update value was persisted and
			// the updated values are seen
			readIxn, ok = entry.(*ServiceIntentionsConfigEntry)
			require.True(t, ok)
			require.Equal(t, "service-intentions", readIxn.Kind)
			require.Equal(t, "foo", readIxn.Name)
			require.Len(t, readIxn.Sources, 2)
			require.Equal(t, map[string]string{"x": "y"}, readIxn.Meta)
		case "bar":
			readIxn, ok = entry.(*ServiceIntentionsConfigEntry)
			require.True(t, ok)
			require.Equal(t, "service-intentions", readIxn.Kind)
			require.Equal(t, "bar", readIxn.Name)
			require.Len(t, readIxn.Sources, 1)
			require.Empty(t, readIxn.Meta)
		}
	}

	// delete one
	wm, err = config_entries.Delete(ServiceIntentions, "foo", nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	require.NotEqual(t, 0, wm.RequestTime)

	// verify deletion
	_, _, err = config_entries.Get(ServiceIntentions, "foo", nil)
	require.Error(t, err)
}
