// +build !consulent

package config

import "github.com/hashicorp/consul/agent/structs"

// EnterpriseMeta provides a stub for the corresponding struct in config_ent.go
type EnterpriseConfig struct{}

// EnterpriseMeta stub
type EnterpriseMeta struct{}

func (_ *EnterpriseMeta) ToStructs() structs.EnterpriseMeta {
	return *structs.DefaultEnterpriseMeta()
}
