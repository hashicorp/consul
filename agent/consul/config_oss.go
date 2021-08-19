// +build !consulent

package consul

import "github.com/hashicorp/consul/agent/structs"

func (c *Config) agentEnterpriseMeta() *structs.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}
