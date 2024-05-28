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
	if !(req.Namespace == "" || req.Namespace == acl.DefaultNamespaceName) || !(req.Partition == acl.DefaultPartitionName || req.Partition == "default") {
		return ErrNotSupported
	}
	return nil
}

func queryTenancyToEntMeta(_ QueryTenancy) acl.EnterpriseMeta {
	return acl.EnterpriseMeta{}
}
