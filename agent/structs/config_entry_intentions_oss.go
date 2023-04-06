// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

import (
	"github.com/hashicorp/consul/acl"
)

func validateSourceIntentionEnterpriseMeta(_, _ *acl.EnterpriseMeta) error {
	return nil
}
