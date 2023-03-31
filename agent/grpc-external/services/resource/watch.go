// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resource

import (
	"github.com/hashicorp/consul/internal/storage"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func (s *Server) WatchList(req *pbresource.WatchListRequest, stream pbresource.ResourceService_WatchListServer) error {
	// check type exists
	if _, err := s.resolveType(req.Type); err != nil {
		return err
	}

	unversionedType := storage.UnversionedTypeFrom(req.Type)
	watch, err := s.backend.WatchList(
		stream.Context(),
		unversionedType,
		req.Tenancy,
		req.NamePrefix,
	)
	if err != nil {
		return err
	}
	defer watch.Close()

	for {
		event, err := watch.Next(stream.Context())
		if err != nil {
			return err
		}

		// drop versions that don't match
		if event.Resource.Id.Type.GroupVersion != req.Type.GroupVersion {
			continue
		}

		if err = stream.Send(event); err != nil {
			return err
		}
	}
}
