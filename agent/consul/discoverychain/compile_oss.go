//go:build !consulent
// +build !consulent

package discoverychain

import "github.com/hashicorp/consul/agent/structs"

func (c *compiler) GetEnterpriseMeta() *structs.EnterpriseMeta {
	return structs.DefaultEnterpriseMetaInDefaultPartition()
}
