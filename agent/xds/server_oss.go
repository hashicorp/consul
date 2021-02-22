// +build !consulent

package xds

import (
	envoycore "github.com/envoyproxy/go-control-plane/envoy/api/v2/core"

	"github.com/hashicorp/consul/agent/structs"
)

func parseEnterpriseMeta(node *envoycore.Node) *structs.EnterpriseMeta {
	return structs.DefaultEnterpriseMeta()
}
