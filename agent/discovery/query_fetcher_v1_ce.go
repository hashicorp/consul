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

// validateEnterpriseTenancy validates the tenancy fields for an enterprise request to
// make sure that they are either set to an empty string or "default" to align with the behavior
// in CE.
func validateEnterpriseTenancy(req QueryTenancy) error {
	if !(req.Namespace == acl.EmptyNamespaceName || req.Namespace == acl.DefaultNamespaceName) ||
		!(req.Partition == acl.DefaultPartitionName || req.Partition == acl.NonEmptyDefaultPartitionName) {
		return ErrNotSupported
	}
	return nil
}

func queryTenancyToEntMeta(_ QueryTenancy) acl.EnterpriseMeta {
	return acl.EnterpriseMeta{}
}
