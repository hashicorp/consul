//go:build !consulent
// +build !consulent

package pbservice

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommon"
)

func EnterpriseMetaToStructs(_ *pbcommon.EnterpriseMeta) structs.EnterpriseMeta {
	return structs.EnterpriseMeta{}
}

func NewEnterpriseMetaFromStructs(_ structs.EnterpriseMeta) *pbcommon.EnterpriseMeta {
	return &pbcommon.EnterpriseMeta{}
}
