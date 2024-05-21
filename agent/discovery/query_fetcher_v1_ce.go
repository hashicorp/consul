// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package discovery

import (
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
