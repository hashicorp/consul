// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeSlices(t *testing.T) {
	require.Nil(t, MergeSlices[int](nil, nil))
}
