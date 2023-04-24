package cachetype

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/mitchellh/hashstructure"
	"google.golang.org/grpc"

	"github.com/hashicorp/consul/agent/cache"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// Recommended name for registration.
const TrustBundleReadName = "peer-trust-bundle"

type TrustBundleReadRequest struct {
	Request *pbpeering.TrustBundleReadRequest
	structs.QueryOptions
}

func (r *TrustBundleReadRequest) CacheInfo() cache.RequestInfo {
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
		r.Request.Name,
	}, nil)
	if err == nil {
		// If there is an error, we don't set the key. A blank key forces
		// no cache for this request so the request is forwarded directly
		// to the server.
		info.Key = strconv.FormatUint(v, 10)
	}

	return info
}

// TrustBundle supports fetching discovering service instances via prepared
// queries.
type TrustBundle struct {
	RegisterOptionsNoRefresh
	Client TrustBundleReader
}

//go:generate mockery --name TrustBundleReader --inpackage --filename mock_TrustBundleReader_test.go
type TrustBundleReader interface {
	TrustBundleRead(
		ctx context.Context, in *pbpeering.TrustBundleReadRequest, opts ...grpc.CallOption,
	) (*pbpeering.TrustBundleReadResponse, error)
}

func (t *TrustBundle) Fetch(_ cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a TrustBundleReadRequest.
	// We do not need to make a copy of this request type like in other cache types
	// because the RequestInfo is synthetic.
	reqReal, ok := req.(*TrustBundleReadRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Always allow stale - there's no point in hitting leader if the request is
	// going to be served from cache and end up arbitrarily stale anyway. This
	// allows cached service-discover to automatically read scale across all
	// servers too.
	reqReal.QueryOptions.SetAllowStale(true)

	// Fetch
	reply, err := t.Client.TrustBundleRead(external.ContextWithToken(context.Background(), reqReal.Token), reqReal.Request)
	if err != nil {
		return result, err
	}

	result.Value = reply
	result.Index = reply.Index

	return result, nil
}
