//go:build !consulent
// +build !consulent

package pbcommon

import (
	"github.com/hashicorp/consul/agent/structs"
)

var DefaultEnterpriseMeta = EnterpriseMeta{}

func EnterpriseMetaToStructs(_ *EnterpriseMeta) structs.EnterpriseMeta {
	return *structs.DefaultEnterpriseMetaInDefaultPartition()
}

func NewEnterpriseMetaFromStructs(_ structs.EnterpriseMeta) *EnterpriseMeta {
	return &EnterpriseMeta{}
}
