package topology

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeSlices(t *testing.T) {
	require.Nil(t, MergeSlices[int](nil, nil))
}
