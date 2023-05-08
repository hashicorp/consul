package cachetype

import (
	"context"
	"testing"
	"time"

	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func TestPeerings(t *testing.T) {
	client := NewMockPeeringLister(t)
	typ := &Peerings{Client: client}

	resp := &pbpeering.PeeringListResponse{
		Index: 48,
		Peerings: []*pbpeering.Peering{
			{
				Name:                "peer1",
				ID:                  "8ac403cf-6834-412f-9dfe-0ac6e69bd89f",
				PeerServerAddresses: []string{"1.2.3.4"},
				State:               pbpeering.PeeringState_ACTIVE,
			},
		},
	}

	// Expect the proper call.
	// This also returns the canned response above.
	client.On("PeeringList", mock.Anything, mock.Anything).
		Return(resp, nil)

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{}, &PeeringListRequest{
		Request: &pbpeering.PeeringListRequest{},
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestPeerings_PeeringDisabled(t *testing.T) {
	client := NewMockPeeringLister(t)
	typ := &Peerings{Client: client}

	var resp *pbpeering.PeeringListResponse

	// Expect the proper call, but return the peering disabled error
	client.On("PeeringList", mock.Anything, mock.Anything).
		Return(resp, grpcstatus.Error(codes.FailedPrecondition, "peering must be enabled to use this endpoint"))

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{}, &PeeringListRequest{
		Request: &pbpeering.PeeringListRequest{},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.EqualValues(t, 1, result.Index)
	require.NotNil(t, result.Value)
}

func TestPeerings_badReqType(t *testing.T) {
	client := pbpeering.NewPeeringServiceClient(nil)
	typ := &Peerings{Client: client}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
}

// This test asserts that we can continuously poll this cache type, given that it doesn't support blocking.
func TestPeerings_MultipleUpdates(t *testing.T) {
	c := cache.New(cache.Options{})

	client := NewMockPeeringLister(t)

	// On each mock client call to PeeringList we will increment the index by 1
	// to simulate new data arriving.
	resp := &pbpeering.PeeringListResponse{
		Index: uint64(0),
	}

	client.On("PeeringList", mock.Anything, mock.Anything).
		Return(func(ctx context.Context, in *pbpeering.PeeringListRequest, opts ...grpc.CallOption) *pbpeering.PeeringListResponse {
			resp.Index++
			// Avoids triggering the race detection by copying the output
			copyResp, err := copystructure.Copy(resp)
			require.NoError(t, err)
			output := copyResp.(*pbpeering.PeeringListResponse)
			return output
		}, nil)

	c.RegisterType(PeeringListName, &Peerings{Client: client})

	ch := make(chan cache.UpdateEvent)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	require.NoError(t, c.Notify(ctx, PeeringListName, &PeeringListRequest{
		Request: &pbpeering.PeeringListRequest{},
	}, "updates", ch))

	i := uint64(1)
	for {
		select {
		case <-ctx.Done():
			t.Fatal("context deadline exceeded")
			return
		case update := <-ch:
			// Expect to receive updates for increasing indexes serially.
			actual := update.Result.(*pbpeering.PeeringListResponse)
			require.Equal(t, i, actual.Index)
			i++

			if i > 3 {
				return
			}
		}
	}
}
