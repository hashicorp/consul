// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package connect

import (
	"github.com/hashicorp/consul/acl"
)

// TODO: this will need to somehow be updated to set namespace here when we include namespaces in CE

// GetEnterpriseMeta will synthesize an EnterpriseMeta struct from the SpiffeIDWorkloadIdentity.
// in CE this just returns an empty (but never nil) struct pointer
func (id SpiffeIDWorkloadIdentity) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &acl.EnterpriseMeta{}
}
