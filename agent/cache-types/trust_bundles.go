package cachetype

import (
	"context"
	"fmt"
	"strconv"

	"github.com/mitchellh/hashstructure"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/cache"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// Recommended name for registration.
const TrustBundleListName = "trust-bundles"

// TrustBundleListRequest represents the combination of request payload
// and options that would normally be sent over headers.
type TrustBundleListRequest struct {
	Request *pbpeering.TrustBundleListByServiceRequest
	structs.QueryOptions
}

func (r *TrustBundleListRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     "",
		MinIndex:       r.MinQueryIndex,
		Timeout:        r.MaxQueryTime,
		MaxAge:         r.MaxAge,
		MustRevalidate: r.MustRevalidate,
	}

	v, err := hashstructure.Hash([]interface{}{
		r.Request.Partition,
		r.Request.Namespace,
		r.Request.ServiceName,
		r.Request.Kind,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// TrustBundles supports fetching discovering service instances via prepared
// queries.
type TrustBundles struct {
	RegisterOptionsBlockingRefresh
	Client TrustBundleLister
}

//go:generate mockery --name TrustBundleLister --inpackage --filename mock_TrustBundleLister_test.go
type TrustBundleLister interface {
	TrustBundleListByService(
		ctx context.Context, in *pbpeering.TrustBundleListByServiceRequest, opts ...grpc.CallOption,
	) (*pbpeering.TrustBundleListByServiceResponse, error)
}

func (t *TrustBundles) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a TrustBundleListRequest.
	// We do not need to make a copy of this request type like in other cache types
	// because the RequestInfo is synthetic.
	reqReal, ok := req.(*TrustBundleListRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Lightweight copy this object so that manipulating QueryOptions doesn't race.
	dup := *reqReal
	reqReal = &dup

	// Set the minimum query index to our current index, so we block
	reqReal.QueryOptions.MinQueryIndex = opts.MinIndex
	reqReal.QueryOptions.MaxQueryTime = opts.Timeout

	// Always allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and end up arbitrarily stale anyway. This
	// allows cached service-discover to automatically read scale across all
	// servers too.
	reqReal.QueryOptions.SetAllowStale(true)

	// Fetch
	ctx, err := external.ContextWithQueryOptions(context.Background(), reqReal.QueryOptions)
	if err != nil {
		return result, err
	}

	var header metadata.MD
	reply, err := t.Client.TrustBundleListByService(ctx, reqReal.Request, grpc.Header(&header))
	if err != nil {
		// Return an empty result if the error is due to peering being disabled.
		// This allows mesh gateways to receive an update and confirm that the watch is set.
		if e, ok := status.FromError(err); ok && e.Code() == codes.FailedPrecondition {
			result.Index = 1
			result.Value = &pbpeering.TrustBundleListByServiceResponse{OBSOLETE_Index: 1}
			return result, nil
		}
		return result, err
	}

	// This first case is using the legacy index field
	// It should be removed in a future version in favor of the index from QueryMeta
	if reply.OBSOLETE_Index != 0 {
		result.Index = reply.OBSOLETE_Index
	} else {
		meta, err := external.QueryMetaFromGRPCMeta(header)
		if err != nil {
			return result, fmt.Errorf("could not convert gRPC metadata to query meta: %w", err)
		}
		result.Index = meta.GetIndex()
	}

	result.Value = reply

	return result, nil
}
