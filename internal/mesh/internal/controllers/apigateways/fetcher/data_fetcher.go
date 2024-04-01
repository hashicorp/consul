// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fetcher

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

type Fetcher struct {
	client pbresource.ResourceServiceClient
}

func New(client pbresource.ResourceServiceClient) *Fetcher {
	return &Fetcher{
		client: client,
	}
}

// method on fetcher to fetch an apigateway
func (f *Fetcher) FetchAPIGateway(ctx context.Context, id *pbresource.ID) (*types.DecodedAPIGateway, error) {
	assertResourceType(pbmesh.APIGatewayType, id.Type)

	dec, err := resource.GetDecodedResource[*pbmesh.APIGateway](ctx, f.client, id)
	if err != nil {
		return nil, err
	}

	return dec, nil
}

func assertResourceType(expected, actual *pbresource.Type) {
	if !proto.Equal(expected, actual) {
		// this is always a programmer error so safe to panic
		panic(fmt.Sprintf("expected a query for a type of %q, you provided a type of %q", expected, actual))
	}
}
