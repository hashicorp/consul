//go:build !consulent
// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func (c *Config) AgentEnterpriseMeta() *acl.EnterpriseMeta {
	return structs.NodeEnterpriseMetaInDefaultPartition()
}
