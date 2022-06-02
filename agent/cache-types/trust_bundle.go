package cachetype

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/proto/pbpeering"
	"google.golang.org/grpc"
)

// Recommended name for registration.
const TrustBundleReadName = "peer-trust-bundle"

// TrustBundle supports fetching discovering service instances via prepared
// queries.
type TrustBundle struct {
	RegisterOptionsNoRefresh
	Client TrustBundleReader
}

//go:generate mockery --name TrustBundleReader --inpackage --testonly
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
	reqReal, ok := req.(*pbpeering.TrustBundleReadRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// Fetch
	reply, err := t.Client.TrustBundleRead(context.Background(), reqReal)
	if err != nil {
		return result, err
	}

	result.Value = reply
	result.Index = reply.Index

	return result, nil
}
