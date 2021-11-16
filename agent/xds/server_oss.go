//go:build !consulent
// +build !consulent

package xds

import (
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"

	"github.com/hashicorp/consul/agent/structs"
)

func parseEnterpriseMeta(node *envoy_core_v3.Node) *structs.EnterpriseMeta {
	return structs.DefaultEnterpriseMetaInDefaultPartition()
}
