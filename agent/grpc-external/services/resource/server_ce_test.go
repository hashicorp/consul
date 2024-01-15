// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package resource_test

import "github.com/hashicorp/consul/acl"

func fillEntMeta(entMeta *acl.EnterpriseMeta) {
	return
}

func fillAuthorizerContext(authzContext *acl.AuthorizerContext) {
	return
}
