//go:build !consulent
// +build !consulent

package pbservice

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto/pbcommon"
)

func EnterpriseMetaToStructs(_ *pbcommon.EnterpriseMeta) acl.EnterpriseMeta {
	return acl.EnterpriseMeta{}
}

func NewEnterpriseMetaFromStructs(_ acl.EnterpriseMeta) *pbcommon.EnterpriseMeta {
	return &pbcommon.EnterpriseMeta{}
}
