// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package testing

import (
	"github.com/hashicorp/consul/acl"
)

func FillEntMeta(entMeta *acl.EnterpriseMeta) {
	// nothing to do in CE.
}

func FillAuthorizerContext(authzContext *acl.AuthorizerContext) {
	// nothing to do in CE.
}
