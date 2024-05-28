// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import (
	"github.com/stretchr/testify/require"
	"testing"
)

const (
	defaultTestNamespace = ""
	defaultTestPartition = ""
)

func Test_validateEnterpriseTenancy(t *testing.T) {
	testCases := []struct {
		name     string
		req      QueryTenancy
		expected error
	}{
		{
			name: "empty namespace and partition returns no error",
			req: QueryTenancy{
				Namespace: defaultTestNamespace,
				Partition: defaultTestPartition,
			},
			expected: nil,
		},
		{
			name: "namespace and partition set to 'default' returns no error",
			req: QueryTenancy{
				Namespace: "default",
				Partition: "default",
			},
			expected: nil,
		},
		{
			name: "namespace set to something other than empty string or `default` returns not supported error",
			req: QueryTenancy{
				Namespace: "namespace-1",
				Partition: "default",
			},
			expected: ErrNotSupported,
		},
		{
			name: "partition set to something other than empty string or `default` returns not supported error",
			req: QueryTenancy{
				Namespace: "default",
				Partition: "partition-1",
			},
			expected: ErrNotSupported,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateEnterpriseTenancy(tc.req)
			require.Equal(t, tc.expected, err)
		})
	}
}
