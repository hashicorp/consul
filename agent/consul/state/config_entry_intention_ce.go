//go:build !consulent
// +build !consulent

package state

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func getIntentionPrecedenceMatchServiceNames(serviceName string, entMeta *acl.EnterpriseMeta) []structs.ServiceName {
	if serviceName == structs.WildcardSpecifier {
		return []structs.ServiceName{
			structs.NewServiceName(structs.WildcardSpecifier, entMeta),
		}
	}

	return []structs.ServiceName{
		structs.NewServiceName(serviceName, entMeta),
		structs.NewServiceName(structs.WildcardSpecifier, entMeta),
	}
}
