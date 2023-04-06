// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"context"

	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) List(ctx context.Context, req *pbresource.ListRequest) (*pbresource.ListResponse, error) {
	if _, err := s.resolveType(req.Type); err != nil {
		return nil, err
	}

	resources, err := s.Backend.List(ctx, readConsistencyFrom(ctx), storage.UnversionedTypeFrom(req.Type), req.Tenancy, req.NamePrefix)
	if err != nil {
		return nil, err
	}

	// filter out non-matching GroupVersion
	result := make([]*pbresource.Resource, 0)
	for _, resource := range resources {
		if resource.Id.Type.GroupVersion == req.Type.GroupVersion {
			result = append(result, resource)
		}
	}
	return &pbresource.ListResponse{Resources: result}, nil
}
