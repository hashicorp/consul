package external

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestQueryOptionsFromContextRoundTrip(t *testing.T) {

	expected := structs.QueryOptions{
		Token: "123",
	}

	ctx, err := ContextWithQueryOptions(context.Background(), expected)
	if err != nil {
		t.Fatal(err)
	}

	out, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatalf("cannot get metadata from context")
	}
	ctx = metadata.NewIncomingContext(ctx, out)

	actual, err := QueryOptionsFromContext(ctx)
	if err != nil {
		t.Fatal(err)
	}

	require.Equal(t, expected, actual)
}
