package finder

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestIDFromArgs(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	client := a.Client()

	// Create a set of intentions
	var ids []string
	{
		insert := [][]string{
			{"a", "b"},
		}

		for _, v := range insert {
			ixn := &api.Intention{
				SourceName:      v[0],
				DestinationName: v[1],
				Action:          api.IntentionActionAllow,
			}

			id, _, err := client.Connect().IntentionCreate(ixn, nil)
			require.NoError(t, err)
			ids = append(ids, id)
		}
	}

	id, err := IDFromArgs(client, []string{"a", "b"})
	require.NoError(t, err)
	require.Equal(t, ids[0], id)

	_, err = IDFromArgs(client, []string{"c", "d"})
	require.Error(t, err)
}
