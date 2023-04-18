package cachetype

import (
	"context"
	"fmt"
	"strconv"
	"time"

	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/proto/pbpeering"
	"github.com/mitchellh/hashstructure"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/structs"
)

// PeeringListName is the recommended name for registration.
const PeeringListName = "peers"

type PeeringListRequest struct {
	Request *pbpeering.PeeringListRequest
	structs.QueryOptions
}

func (r *PeeringListRequest) CacheInfo() cache.RequestInfo {
	info := cache.RequestInfo{
		Token:          r.Token,
		Datacenter:     "",
		MinIndex:       0,
		Timeout:        0,
		MustRevalidate: false,

		// OPTIMIZE(peering): Cache.notifyPollingQuery polls at this interval. We need to revisit how that polling works.
		//        	          Using an exponential backoff when the result hasn't changed may be preferable.
		MaxAge: 1 * time.Second,
	}

	v, err := hashstructure.Hash([]interface{}{
		r.Request.Partition,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// Peerings supports fetching the list of peers for a given partition or wildcard-specifier.
type Peerings struct {
	RegisterOptionsNoRefresh
	Client PeeringLister
}

//go:generate mockery --name PeeringLister --inpackage --filename mock_PeeringLister_test.go
type PeeringLister interface {
	PeeringList(
		ctx context.Context, in *pbpeering.PeeringListRequest, opts ...grpc.CallOption,
	) (*pbpeering.PeeringListResponse, error)
}

func (t *Peerings) Fetch(_ cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a PeeringListRequest.
	// We do not need to make a copy of this request type like in other cache types
	// because the RequestInfo is synthetic.
	reqReal, ok := req.(*PeeringListRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Always allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and end up arbitrarily stale anyway. This
	// allows cached service-discover to automatically read scale across all
	// servers too.
	reqReal.QueryOptions.SetAllowStale(true)

	ctx, err := external.ContextWithQueryOptions(context.Background(), reqReal.QueryOptions)
	if err != nil {
		return result, err
	}

	// Fetch
	reply, err := t.Client.PeeringList(ctx, reqReal.Request)
	if err != nil {
		// Return an empty result if the error is due to peering being disabled.
		// This allows mesh gateways to receive an update and confirm that the watch is set.
		if e, ok := status.FromError(err); ok && e.Code() == codes.FailedPrecondition {
			result.Index = 1
			result.Value = &pbpeering.PeeringListResponse{}
			return result, nil
		}
		return result, err
	}

	result.Value = reply
	result.Index = reply.Index

	return result, nil
}
