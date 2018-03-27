package api

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// NOTE(mitchellh): we don't have a way to test CA roots yet since there
// is no API public way to configure the root certs. This wll be resolved
// in the future and we can write tests then. This is tested in agent and
// agent/consul which do have internal access to manually create roots.

func TestAPI_ConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	c, s := makeClient(t)
	defer s.Stop()

	connect := c.Connect()
	list, meta, err := connect.CARoots(nil)
	require.Nil(err)
	require.Equal(uint64(0), meta.LastIndex)
	require.Len(list.Roots, 0)
}
