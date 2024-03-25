// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/acl"
)

func (f *V1DataFetcher) NormalizeRequest(req *QueryPayload) {
	// Nothing to do for CE
	return
}

func validateEnterpriseTenancy(req QueryTenancy) error {
	if req.Namespace != "" || req.Partition != acl.DefaultPartitionName {
		return ErrNotSupported
	}
	return nil
}

func queryTenancyToEntMeta(_ QueryTenancy) acl.EnterpriseMeta {
	return acl.EnterpriseMeta{}
}

// fetchServiceFromSamenessGroup fetches a service from a sameness group.
func (f *V1DataFetcher) fetchServiceFromSamenessGroup(ctx Context, req *QueryPayload, cfg *V1DataFetcherDynamicConfig, lookupType LookupType) ([]*Result, error) {
	f.logger.Trace(fmt.Sprintf("fetchServiceFromSamenessGroup - req: %+v", req))
	if req.Tenancy.SamenessGroup == "" {
		return nil, errors.New("sameness groups must be provided for service lookups")
	}
	return f.fetchServiceBasedOnTenancy(ctx, req, cfg, lookupType)
}
