// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package dns

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestNewContextFromGRPCContext(t *testing.T) {
	t.Parallel()

	md := metadata.MD{}
	testMeta := map[string]string{
		"x-consul-token":     "test-token",
		"x-consul-namespace": "test-namespace",
		"x-consul-partition": "test-partition",
	}

	for k, v := range testMeta {
		md.Set(k, v)
	}
	testGRPCContext := metadata.NewIncomingContext(context.Background(), md)

	testCases := []struct {
		name     string
		grpcCtx  context.Context
		expected *Context
		error    error
	}{
		{
			name:     "nil grpc context",
			grpcCtx:  nil,
			expected: &Context{},
		},
		{
			name:     "grpc context w/o metadata",
			grpcCtx:  context.Background(),
			expected: &Context{},
		},
		{
			name:    "grpc context w/ kitchen sink",
			grpcCtx: testGRPCContext,
			expected: &Context{
				Token:            "test-token",
				DefaultNamespace: "test-namespace",
				DefaultPartition: "test-partition",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := NewContextFromGRPCContext(tc.grpcCtx)
			if tc.error != nil {
				require.Error(t, err)
				require.Equal(t, Context{}, &ctx)
				require.Equal(t, tc.error, err)
				return
			}

			require.NotNil(t, ctx)
			require.Equal(t, tc.expected, &ctx)
		})
	}
}
