// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import (
	"errors"
	"fmt"
)

// fetchServiceFromSamenessGroup fetches a service from a sameness group.
func (f *V1DataFetcher) fetchServiceFromSamenessGroup(ctx Context, req *QueryPayload, cfg *v1DataFetcherDynamicConfig) ([]*Result, error) {
	f.logger.Debug(fmt.Sprintf("fetchServiceFromSamenessGroup - req: %+v", req))
	if req.Tenancy.SamenessGroup == "" {
		return nil, errors.New("sameness groups must be provided for service lookups")
	}
	return f.fetchServiceBasedOnTenancy(ctx, req, cfg)
}
