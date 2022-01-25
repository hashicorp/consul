//go:build !consulent
// +build !consulent

package consul

import "github.com/hashicorp/consul/agent/structs"

func (c *Config) AgentEnterpriseMeta() *structs.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}
