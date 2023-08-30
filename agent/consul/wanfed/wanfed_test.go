// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package wanfed

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitNodeName(t *testing.T) {
	type testcase struct {
		nodeName        string
		expectShortName string
		expectDC        string
		expectErr       bool
	}

	cases := []testcase{
		// bad
		{nodeName: "", expectErr: true},
		{nodeName: "foo", expectErr: true},
		{nodeName: "foo.bar.baz", expectErr: true},
		// good
		{nodeName: "foo.bar", expectShortName: "foo", expectDC: "bar"},
		// weird
		{nodeName: ".bar", expectShortName: "", expectDC: "bar"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.nodeName, func(t *testing.T) {
			gotShortName, gotDC, gotErr := SplitNodeName(tc.nodeName)
			if tc.expectErr {
				require.Error(t, gotErr)
				require.Empty(t, gotShortName)
				require.Empty(t, gotDC)
			} else {
				require.NoError(t, gotErr)
				require.Equal(t, tc.expectShortName, gotShortName)
				require.Equal(t, tc.expectDC, gotDC)
			}
		})
	}
}
