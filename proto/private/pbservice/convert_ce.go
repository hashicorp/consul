// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package pbservice

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/private/pbcommon"
)

func EnterpriseMetaToStructs(_ *pbcommon.EnterpriseMeta) acl.EnterpriseMeta {
	return acl.EnterpriseMeta{}
}

func NewEnterpriseMetaFromStructs(_ acl.EnterpriseMeta) *pbcommon.EnterpriseMeta {
	return &pbcommon.EnterpriseMeta{}
}
