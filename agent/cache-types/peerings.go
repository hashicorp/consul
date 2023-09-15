// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

// PeeringListName is the recommended name for registration.
const PeeringListName = "peers"

// PeeringListRequest represents the combination of request payload
// and options that would normally be sent over headers.
type PeeringListRequest struct {
	Request *pbpeering.PeeringListRequest
	structs.QueryOptions
}

func (r *PeeringListRequest) CacheInfo() cache.RequestInfo {
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
	RegisterOptionsBlockingRefresh
	Client PeeringLister
}

//go:generate mockery --name PeeringLister --inpackage --filename mock_PeeringLister_test.go
type PeeringLister interface {
	PeeringList(
		ctx context.Context, in *pbpeering.PeeringListRequest, opts ...grpc.CallOption,
	) (*pbpeering.PeeringListResponse, error)
}

func (t *Peerings) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// The request should be a PeeringListRequest.
	// We do not need to make a copy of this request type like in other cache types
	// because the RequestInfo is synthetic.
	reqReal, ok := req.(*PeeringListRequest)
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

	// We allow stale queries here to spread out the RPC load, but peerstream information, including the STATUS,
	// will not be returned. Right now this is fine for the watch in proxycfg/mesh_gateway.go,
	// but it could be a problem for a future consumer.
	reqReal.QueryOptions.SetAllowStale(true)

	ctx, err := external.ContextWithQueryOptions(context.Background(), reqReal.QueryOptions)
	if err != nil {
		return result, err
	}

	// Fetch
	var header metadata.MD
	reply, err := t.Client.PeeringList(ctx, reqReal.Request, grpc.Header(&header))
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
