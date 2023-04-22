// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package connect

import (
	"fmt"

	"github.com/hashicorp/consul/acl"
)

// GetEnterpriseMeta will synthesize an EnterpriseMeta struct from the SpiffeIDAgent.
// in OSS this just returns an empty (but never nil) struct pointer
func (id SpiffeIDMeshGateway) GetEnterpriseMeta() *acl.EnterpriseMeta {
	return &acl.EnterpriseMeta{}
}

func (id SpiffeIDMeshGateway) uriPath() string {
	return fmt.Sprintf("/gateway/mesh/dc/%s", id.Datacenter)
}
