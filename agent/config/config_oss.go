//go:build !consulent
// +build !consulent

package config

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (_ *EnterpriseMeta) ToStructs() acl.EnterpriseMeta {
	return *structs.DefaultEnterpriseMetaInDefaultPartition()
}
