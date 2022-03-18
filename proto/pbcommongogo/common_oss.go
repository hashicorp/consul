//go:build !consulent
// +build !consulent

package pbcommongogo

import (
	"github.com/hashicorp/consul/agent/structs"
)

var DefaultEnterpriseMeta = EnterpriseMeta{}

func NewEnterpriseMetaFromStructs(_ structs.EnterpriseMeta) *EnterpriseMeta {
	return &EnterpriseMeta{}
}
