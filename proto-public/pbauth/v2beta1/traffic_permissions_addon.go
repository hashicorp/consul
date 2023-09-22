// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package authv2beta1

type SourceToSpiffe interface {
	GetIdentityName() string
	GetPartition() string
	GetNamespace() string
	GetPeer() string
}

var _ SourceToSpiffe = (*Source)(nil)
var _ SourceToSpiffe = (*ExcludeSource)(nil)
