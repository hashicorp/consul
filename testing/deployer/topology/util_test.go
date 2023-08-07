// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package topology

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeSlices(t *testing.T) {
	require.Nil(t, MergeSlices[int](nil, nil))
}
