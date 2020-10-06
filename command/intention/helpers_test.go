package intention

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/stretchr/testify/require"
)

func TestGetFromArgs(t *testing.T) {
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

	t.Run("l4 intention", func(t *testing.T) {
		t.Run("one arg", func(t *testing.T) {
			ixn, err := GetFromArgs(client, []string{id0})
			require.NoError(t, err)
			require.Equal(t, id0, ixn.ID)
			require.Equal(t, "a", ixn.SourceName)
			require.Equal(t, "b", ixn.DestinationName)
			require.Equal(t, api.IntentionActionAllow, ixn.Action)
		})
		t.Run("two args", func(t *testing.T) {
			ixn, err := GetFromArgs(client, []string{"a", "b"})
			require.NoError(t, err)
			require.Equal(t, id0, ixn.ID)
			require.Equal(t, "a", ixn.SourceName)
			require.Equal(t, "b", ixn.DestinationName)
			require.Equal(t, api.IntentionActionAllow, ixn.Action)
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
