// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package external

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/agent/structs"
)

func TestQueryOptionsFromContextRoundTrip(t *testing.T) {

	expected := structs.QueryOptions{
		Token:         "123",
		AllowStale:    true,
		MinQueryIndex: uint64(10),
		MaxAge:        1 * time.Hour,
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
