package api

import (
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/stretchr/testify/require"
)

func TestAPI_ConnectCARoots_empty(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	c, s := makeClientWithConfig(t, nil, func(c *testutil.TestServerConfig) {
		// Don't bootstrap CA
		c.Connect = nil
	})
	defer s.Stop()

	connect := c.Connect()
	list, meta, err := connect.CARoots(nil)
	require.NoError(err)
	require.Equal(uint64(0), meta.LastIndex)
	require.Len(list.Roots, 0)
	require.Empty(list.TrustDomain)
}

func TestAPI_ConnectCARoots_list(t *testing.T) {
	t.Parallel()

	c, s := makeClient(t)
	defer s.Stop()

	// This fails occasionally if server doesn't have time to bootstrap CA so
	// retry
	retry.Run(t, func(r *retry.R) {
		connect := c.Connect()
		list, meta, err := connect.CARoots(nil)
		r.Check(err)
		if meta.LastIndex <= 0 {
			r.Fatalf("expected roots raft index to be > 0")
		}
		if v := len(list.Roots); v != 1 {
			r.Fatalf("expected 1 root, got %d", v)
		}
		// connect.TestClusterID causes import cycle so hard code it
		if list.TrustDomain != "11111111-2222-3333-4444-555555555555.consul" {
			r.Fatalf("expected fixed trust domain got '%s'", list.TrustDomain)
		}
	})

}
