// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package testing

import (
	"github.com/hashicorp/consul/acl"
)

func FillEntMeta(entMeta *acl.EnterpriseMeta) {
	// nothing to to in CE.
}

func FillAuthorizerContext(authzContext *acl.AuthorizerContext) {
	// nothing to to in CE.
}
