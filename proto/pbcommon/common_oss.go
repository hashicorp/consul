//go:build !consulent
// +build !consulent

package pbcommon

import (
	"github.com/hashicorp/consul/agent/structs"
)

var DefaultEnterpriseMeta = &EnterpriseMeta{}

func NewEnterpriseMetaFromStructs(_ structs.EnterpriseMeta) *EnterpriseMeta {
	return &EnterpriseMeta{}
}
func EnterpriseMetaToStructs(s *EnterpriseMeta, t *structs.EnterpriseMeta) {
	if s == nil {
		return
	}
}
func EnterpriseMetaFromStructs(t *structs.EnterpriseMeta, s *EnterpriseMeta) {
	if s == nil {
		return
	}
}
