// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package resource

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// AuthorizerContext builds an ACL AuthorizerContext for the given tenancy.
func AuthorizerContext(t *pbresource.Tenancy) *acl.AuthorizerContext {
	return &acl.AuthorizerContext{Peer: t.PeerName}
}
