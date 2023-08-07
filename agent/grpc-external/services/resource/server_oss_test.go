// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package resource

import "github.com/hashicorp/consul/acl"

func fillEntMeta(entMeta *acl.EnterpriseMeta) {
	return
}

func fillAuthorizerContext(authzContext *acl.AuthorizerContext) {
	return
}
