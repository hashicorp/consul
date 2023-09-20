package external

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
)

func TestQueryMetaFromGRPCMetaRoundTrip(t *testing.T) {
	lastContact, err := time.ParseDuration("1s")
	require.NoError(t, err)

	expected := structs.QueryMeta{
		Index:                 42,
		LastContact:           lastContact,
		KnownLeader:           true,
		ConsistencyLevel:      "stale",
		NotModified:           true,
		Backend:               structs.QueryBackend(0),
		ResultsFilteredByACLs: true,
	}

	md, err := GRPCMetadataFromQueryMeta(expected)
	require.NoError(t, err)

	actual, err := QueryMetaFromGRPCMeta(md)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, expected, actual)
}
