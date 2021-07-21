// +build !consulent

package config

import "github.com/hashicorp/consul/agent/structs"

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (_ *EnterpriseMeta) ToStructs() structs.EnterpriseMeta {
	return *structs.DefaultEnterpriseMetaInDefaultPartition()
}
