package finder

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/require"
)

func TestFinder(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	// Create a set of intentions
	var ids []string
	{
		insert := [][]string{
			[]string{"a", "b", "c", "d"},
		}

		for _, v := range insert {
			ixn := &api.Intention{
				SourceNS:        v[0],
				SourceName:      v[1],
				DestinationNS:   v[2],
				DestinationName: v[3],
				Action:          api.IntentionActionAllow,
			}

			id, _, err := client.Connect().IntentionCreate(ixn, nil)
			require.NoError(err)
			ids = append(ids, id)
		}
	}

	finder := &Finder{Client: client}
	ixn, err := finder.Find("a/b", "c/d")
	require.NoError(err)
	require.Equal(ids[0], ixn.ID)

	ixn, err = finder.Find("a/c", "c/d")
	require.NoError(err)
	require.Nil(ixn)
}
