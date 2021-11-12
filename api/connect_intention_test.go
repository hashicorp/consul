package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConnectIntentionCreateListGetUpdateDelete(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForServiceIntentions(t)

	connect := c.Connect()

	// Create
	ixn := testIntention()
	id, _, err := connect.IntentionCreate(ixn, nil)
	require.Nil(t, err)
	require.NotEmpty(t, id)

	// List it
	list, _, err := connect.Intentions(nil)
	require.Nil(t, err)
	require.Len(t, list, 1)

	actual := list[0]
	ixn.ID = id
	ixn.CreatedAt = actual.CreatedAt
	ixn.UpdatedAt = actual.UpdatedAt
	ixn.CreateIndex = actual.CreateIndex
	ixn.ModifyIndex = actual.ModifyIndex
	ixn.SourcePartition = actual.SourcePartition
	ixn.DestinationPartition = actual.DestinationPartition
	ixn.Hash = actual.Hash
	require.Equal(t, ixn, actual)

	// Get it
	actual, _, err = connect.IntentionGet(id, nil)
	require.Nil(t, err)
	require.Equal(t, ixn, actual)

	// Update it
	ixn.SourceName = ixn.SourceName + "-different"
	_, err = connect.IntentionUpdate(ixn, nil)
	require.NoError(t, err)

	// Get it
	actual, _, err = connect.IntentionGet(id, nil)
	require.NoError(t, err)
	ixn.UpdatedAt = actual.UpdatedAt
	ixn.ModifyIndex = actual.ModifyIndex
	ixn.Hash = actual.Hash
	require.Equal(t, ixn, actual)

	// Delete it
	_, err = connect.IntentionDelete(id, nil)
	require.Nil(t, err)

	// Get it (should be gone)
	actual, _, err = connect.IntentionGet(id, nil)
	require.Nil(t, err)
	require.Nil(t, actual)
}

func TestAPI_ConnectIntentionGet_invalidId(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForServiceIntentions(t)

	connect := c.Connect()

	// Get it
	actual, _, err := connect.IntentionGet("hello", nil)
	require.Nil(t, actual)
	require.Error(t, err)
	require.Contains(t, err.Error(), "UUID") // verify it contains the message
}

func TestAPI_ConnectIntentionMatch(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForServiceIntentions(t)

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
			require.Nil(t, err)
			require.NotEmpty(t, id)
		}
	}

	// Match it
	result, _, err := connect.IntentionMatch(&IntentionMatch{
		By:    IntentionMatchDestination,
		Names: []string{"bar"},
	}, nil)
	require.Nil(t, err)
	require.Len(t, result, 1)

	var actual [][]string
	expected := [][]string{
		{"default", "bar"},
		{"default", "*"},
	}
	for _, ixn := range result["bar"] {
		actual = append(actual, []string{ixn.DestinationNS, ixn.DestinationName})
	}

	require.Equal(t, expected, actual)
}

func TestAPI_ConnectIntentionCheck(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	s.WaitForServiceIntentions(t)

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
			require.Nil(t, err)
			require.NotEmpty(t, id)
		}
	}

	// Match the deny rule
	{
		result, _, err := connect.IntentionCheck(&IntentionCheck{
			Source:      "default/qux",
			Destination: "default/bar",
		}, nil)
		require.NoError(t, err)
		require.False(t, result)
	}

	// Match the allow rule
	{
		result, _, err := connect.IntentionCheck(&IntentionCheck{
			Source:      "default/foo",
			Destination: "default/bar",
		}, nil)
		require.NoError(t, err)
		require.True(t, result)
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
	}
}
