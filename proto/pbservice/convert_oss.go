//go:build !consulent
// +build !consulent

package pbservice

import (
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbcommongogo"
)

func EnterpriseMetaToStructs(_ pbcommongogo.EnterpriseMeta) structs.EnterpriseMeta {
	return structs.EnterpriseMeta{}
}

func NewEnterpriseMetaFromStructs(_ structs.EnterpriseMeta) pbcommongogo.EnterpriseMeta {
	return pbcommongogo.EnterpriseMeta{}
}
