package intention

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestGetFromArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Create some intentions.

	//nolint:staticcheck
	id0, _, err := client.Connect().IntentionCreate(&api.Intention{
		SourceName:      "a",
		DestinationName: "b",
		Action:          api.IntentionActionAllow,
	}, nil)
	require.NoError(t, err)

	// Ensure "y" is L7
	_, _, err = client.ConfigEntries().Set(&api.ServiceConfigEntry{
		Kind:     api.ServiceDefaults,
		Name:     "y",
		Protocol: "http",
	}, nil)
	require.NoError(t, err)

	_, err = client.Connect().IntentionUpsert(&api.Intention{
		SourceName:      "x",
		DestinationName: "y",
		Permissions: []*api.IntentionPermission{
			{
				Action: api.IntentionActionAllow,
				HTTP: &api.IntentionHTTPPermission{
					PathExact: "/foo",
				},
			},
		},
	}, nil)
	require.NoError(t, err)

	t.Run("l4 intention", func(t *testing.T) {
		t.Run("one arg", func(t *testing.T) {
			ixn, err := GetFromArgs(client, []string{id0})
			require.NoError(t, err)
			require.Equal(t, id0, ixn.ID)
			require.Equal(t, "a", ixn.SourceName)
			require.Equal(t, "b", ixn.DestinationName)
			require.Equal(t, api.IntentionActionAllow, ixn.Action)
			require.Empty(t, ixn.Permissions)
		})
		t.Run("two args", func(t *testing.T) {
			ixn, err := GetFromArgs(client, []string{"a", "b"})
			require.NoError(t, err)
			require.Equal(t, id0, ixn.ID)
			require.Equal(t, "a", ixn.SourceName)
			require.Equal(t, "b", ixn.DestinationName)
			require.Equal(t, api.IntentionActionAllow, ixn.Action)
			require.Empty(t, ixn.Permissions)
		})
	})

	t.Run("l7 intention", func(t *testing.T) {
		t.Run("two args", func(t *testing.T) {
			ixn, err := GetFromArgs(client, []string{"x", "y"})
			require.NoError(t, err)
			require.Empty(t, ixn.ID)
			require.Equal(t, "x", ixn.SourceName)
			require.Equal(t, "y", ixn.DestinationName)
			require.Empty(t, ixn.Action)
			require.Equal(t, []*api.IntentionPermission{{
				Action: api.IntentionActionAllow,
				HTTP: &api.IntentionHTTPPermission{
					PathExact: "/foo",
				},
			}}, ixn.Permissions)
		})
	})

	t.Run("missing intention", func(t *testing.T) {
		t.Run("one arg", func(t *testing.T) {
			fakeID := "59208cab-b431-422e-87dc-290b18513082"
			_, err := GetFromArgs(client, []string{fakeID})
			require.Error(t, err)
		})
		t.Run("two args", func(t *testing.T) {
			_, err := GetFromArgs(client, []string{"c", "d"})
			require.Error(t, err)
		})
	})
}
