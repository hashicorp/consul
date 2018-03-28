package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPI_ConnectIntentionCreate(t *testing.T) {
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
	require.Equal(ixn, actual)
}

func testIntention() *Intention {
	return &Intention{
		SourceNS:        "eng",
		SourceName:      "api",
		DestinationNS:   "eng",
		DestinationName: "db",
		Action:          IntentionActionAllow,
		SourceType:      IntentionSourceConsul,
		Meta:            map[string]string{},
	}
}
