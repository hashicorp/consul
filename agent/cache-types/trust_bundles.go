package cachetype

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/proto/pbpeering"
	"google.golang.org/grpc"
)

// Recommended name for registration.
const TrustBundleListName = "trust-bundles"

// TrustBundles supports fetching discovering service instances via prepared
// queries.
type TrustBundles struct {
	RegisterOptionsNoRefresh
	Client TrustBundleLister
}

type TrustBundleLister interface {
	TrustBundleListByService(
		ctx context.Context, in *pbpeering.TrustBundleListByServiceRequest, opts ...grpc.CallOption,
	) (*pbpeering.TrustBundleListByServiceResponse, error)
}

func (t *TrustBundles) Fetch(_ cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a TrustBundleListByServiceRequest.
	// We do not need to make a copy of this request type like in other cache types
	// because the RequestInfo is synthetic.
	reqReal, ok := req.(*pbpeering.TrustBundleListByServiceRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Fetch
	reply, err := t.Client.TrustBundleListByService(context.Background(), reqReal)
	if err != nil {
		return result, err
	}

	result.Value = reply
	result.Index = reply.Index

	return result, nil
}
