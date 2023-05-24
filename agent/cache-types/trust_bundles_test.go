package cachetype

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/cache"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func TestTrustBundles(t *testing.T) {
	client := NewMockTrustBundleLister(t)
	typ := &TrustBundles{Client: client}

	resp := &pbpeering.TrustBundleListByServiceResponse{
		Bundles: []*pbpeering.PeeringTrustBundle{
			{
				PeerName: "peer1",
				RootPEMs: []string{"peer1-roots"},
			},
		},
	}

	// Expect the proper call.
	// This also returns the canned response above.
	client.On("TrustBundleListByService", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			// Validate Query Options
			ctx := args.Get(0).(context.Context)
			out, ok := metadata.FromOutgoingContext(ctx)
			require.True(t, ok)
			ctx = metadata.NewIncomingContext(ctx, out)

			options, err := external.QueryOptionsFromContext(ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(28), options.MinQueryIndex)
			require.Equal(t, time.Duration(1100), options.MaxQueryTime)
			require.True(t, options.AllowStale)

			// Validate Request
			req := args.Get(1).(*pbpeering.TrustBundleListByServiceRequest)
			require.Equal(t, "foo", req.ServiceName)

			// Send back Query Meta on pointer of header
			header := args.Get(2).(grpc.HeaderCallOption)
			qm := structs.QueryMeta{
				Index: 48,
			}

			md, err := external.GRPCMetadataFromQueryMeta(qm)
			require.NoError(t, err)
			*header.HeaderAddr = md
		}).
		Return(resp, nil)

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{
		MinIndex: 28,
		Timeout:  time.Duration(1100),
	}, &TrustBundleListRequest{
		Request: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
	})
	require.NoError(t, err)
	require.Equal(t, cache.FetchResult{
		Value: resp,
		Index: 48,
	}, result)
}

func TestTrustBundles_PeeringDisabled(t *testing.T) {
	client := NewMockTrustBundleLister(t)
	typ := &TrustBundles{Client: client}

	var resp *pbpeering.TrustBundleListByServiceResponse

	// Expect the proper call.
	// This also returns the canned response above.
	client.On("TrustBundleListByService", mock.Anything, mock.Anything, mock.Anything).
		Return(resp, grpcstatus.Error(codes.FailedPrecondition, "peering must be enabled to use this endpoint"))

	// Fetch and assert against the result.
	result, err := typ.Fetch(cache.FetchOptions{}, &TrustBundleListRequest{
		Request: &pbpeering.TrustBundleListByServiceRequest{
			ServiceName: "foo",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.EqualValues(t, 1, result.Index)
	require.NotNil(t, result.Value)
}

func TestTrustBundles_badReqType(t *testing.T) {
	client := pbpeering.NewPeeringServiceClient(nil)
	typ := &TrustBundles{Client: client}

	// Fetch
	_, err := typ.Fetch(cache.FetchOptions{}, cache.TestRequest(
		t, cache.RequestInfo{Key: "foo", MinIndex: 64}))
	require.Error(t, err)
	require.Contains(t, err.Error(), "wrong type")
}
