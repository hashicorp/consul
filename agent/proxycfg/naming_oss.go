//go:build !consulent
// +build !consulent

package proxycfg

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func UpstreamIDString(typ, dc, name string, _ *acl.EnterpriseMeta, peerName string) string {
	ret := name

	if peerName != "" {
		ret += "?peer=" + peerName
	} else if dc != "" {
		ret += "?dc=" + dc
	}

	if typ == "" || typ == structs.UpstreamDestTypeService {
		return ret
	}

	return typ + ":" + ret
}

func parseInnerUpstreamIDString(input string) (string, *acl.EnterpriseMeta) {
	return input, structs.DefaultEnterpriseMetaInDefaultPartition()
}

func (u UpstreamID) enterpriseIdentifierPrefix() string {
	return ""
}
