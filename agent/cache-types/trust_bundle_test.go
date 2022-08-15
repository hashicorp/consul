package cachetype

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTrustBundle(t *testing.T) {
	client := NewMockTrustBundleReader(t)
	typ := &TrustBundle{Client: client}

	resp := &pbpeering.TrustBundleReadResponse{
		Index: 48,
		Bundle: &pbpeering.PeeringTrustBundle{
			PeerName: "peer1",
			RootPEMs: []string{"peer1-roots"},
		},
	}

	// Expect the proper call.
	// This also returns the canned response above.
	client.On("TrustBundleRead", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*pbpeering.TrustBundleReadRequest)
			require.Equal(t, "foo", req.Name)
		}).
		Return(resp, nil)

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{}, &TrustBundleReadRequest{
		Request: &pbpeering.TrustBundleReadRequest{
			Name: "foo",
		},
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestTrustBundle_badReqType(t *testing.T) {
	client := pbpeering.NewPeeringServiceClient(nil)
	typ := &TrustBundle{Client: client}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
}

// This test asserts that we can continuously poll this cache type, given that it doesn't support blocking.
func TestTrustBundle_MultipleUpdates(t *testing.T) {
	c := cache.New(cache.Options{})

	client := NewMockTrustBundleReader(t)

	// On each mock client call to TrustBundleList by service we will increment the index by 1
	// to simulate new data arriving.
	resp := &pbpeering.TrustBundleReadResponse{
		Index: uint64(0),
	}

	client.On("TrustBundleRead", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			req := args.Get(1).(*pbpeering.TrustBundleReadRequest)
			require.Equal(t, "foo", req.Name)

			// Increment on each call.
			resp.Index++
		}).
		Return(resp, nil)

	c.RegisterType(TrustBundleReadName, &TrustBundle{Client: client})

	ch := make(chan cache.UpdateEvent)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	err := c.Notify(ctx, TrustBundleReadName, &TrustBundleReadRequest{
		Request: &pbpeering.TrustBundleReadRequest{Name: "foo"},
	}, "updates", ch)
	require.NoError(t, err)

	i := uint64(1)
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-ch:
			// Expect to receive updates for increasing indexes serially.
			resp := update.Result.(*pbpeering.TrustBundleReadResponse)
			require.Equal(t, i, resp.Index)
			i++

			if i > 3 {
				return
			}
		}
	}
}
