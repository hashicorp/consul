package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConnectIntentionCreateListGetUpdateDelete(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	c, s := makeClient(t)
	defer s.Stop()

	connect := c.Connect()

	// Create
	ixn := testIntention()
	id, _, err := connect.IntentionCreate(ixn, nil)
	require.Nil(err)
	require.NotEmpty(id)

	// List it
	list, _, err := connect.Intentions(nil)
	require.Nil(err)
	require.Len(list, 1)

	actual := list[0]
	ixn.ID = id
	ixn.CreatedAt = actual.CreatedAt
	ixn.UpdatedAt = actual.UpdatedAt
	ixn.CreateIndex = actual.CreateIndex
	ixn.ModifyIndex = actual.ModifyIndex
	ixn.Hash = actual.Hash
	require.Equal(ixn, actual)

	// Get it
	actual, _, err = connect.IntentionGet(id, nil)
	require.Nil(err)
	require.Equal(ixn, actual)

	// Update it
	ixn.SourceName = ixn.SourceName + "-different"
	_, err = connect.IntentionUpdate(ixn, nil)
	require.NoError(err)

	// Get it
	actual, _, err = connect.IntentionGet(id, nil)
	require.NoError(err)
	ixn.UpdatedAt = actual.UpdatedAt
	ixn.ModifyIndex = actual.ModifyIndex
	ixn.Hash = actual.Hash
	require.Equal(ixn, actual)

	// Delete it
	_, err = connect.IntentionDelete(id, nil)
	require.Nil(err)

	// Get it (should be gone)
	actual, _, err = connect.IntentionGet(id, nil)
	require.Nil(err)
	require.Nil(actual)
}

func TestAPI_ConnectIntentionGet_invalidId(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	c, s := makeClient(t)
	defer s.Stop()

	connect := c.Connect()

	// Get it
	actual, _, err := connect.IntentionGet("hello", nil)
	require.Nil(actual)
	require.Error(err)
	require.Contains(err.Error(), "UUID") // verify it contains the message
}

func TestAPI_ConnectIntentionMatch(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	c, s := makeClient(t)
	defer s.Stop()

	connect := c.Connect()

	// Create
	{
		insert := [][]string{
			{"default", "*"},
			{"default", "bar"},
			{"default", "baz"}, // shouldn't match
		}

		for _, v := range insert {
			ixn := testIntention()
			ixn.DestinationNS = v[0]
			ixn.DestinationName = v[1]
			id, _, err := connect.IntentionCreate(ixn, nil)
			require.Nil(err)
			require.NotEmpty(id)
		}
	}

	// Match it
	result, _, err := connect.IntentionMatch(&IntentionMatch{
		By:    IntentionMatchDestination,
		Names: []string{"bar"},
	}, nil)
	require.Nil(err)
	require.Len(result, 1)

	var actual [][]string
	expected := [][]string{
		{"default", "bar"},
		{"default", "*"},
	}
	for _, ixn := range result["bar"] {
		actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
	}

	require.Equal(expected, actual)
}

func TestAPI_ConnectIntentionCheck(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	c, s := makeClient(t)
	defer s.Stop()

	connect := c.Connect()

	// Create
	{
		insert := [][]string{
			{"default", "*", "default", "bar", "deny"},
			{"default", "foo", "default", "bar", "allow"},
		}

		for _, v := range insert {
			ixn := testIntention()
			ixn.SourceNS = v[0]
			ixn.SourceName = v[1]
			ixn.DestinationNS = v[2]
			ixn.DestinationName = v[3]
			ixn.Action = IntentionAction(v[4])
			id, _, err := connect.IntentionCreate(ixn, nil)
			require.Nil(err)
			require.NotEmpty(id)
		}
	}

	// Match the deny rule
	{
		result, _, err := connect.IntentionCheck(&IntentionCheck{
			Source:      "default/qux",
			Destination: "default/bar",
		}, nil)
		require.NoError(err)
		require.False(result)
	}

	// Match the allow rule
	{
		result, _, err := connect.IntentionCheck(&IntentionCheck{
			Source:      "default/foo",
			Destination: "default/bar",
		}, nil)
		require.NoError(err)
		require.True(result)
	}
}

func testIntention() *Intention {
	return &Intention{
		SourceNS:        "default",
		SourceName:      "api",
		DestinationNS:   "default",
		DestinationName: "db",
		Precedence:      9,
		Action:          IntentionActionAllow,
		SourceType:      IntentionSourceConsul,
		Meta:            map[string]string{},
	}
}
