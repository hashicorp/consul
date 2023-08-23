// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package auth

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
)

func bindEnterpriseMeta(authMethod *structs.ACLAuthMethod, verifiedIdentity *authmethod.Identity) (acl.EnterpriseMeta, error) {
	return acl.EnterpriseMeta{}, nil
}
