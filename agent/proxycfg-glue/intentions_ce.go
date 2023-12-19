//go:build !consulent
// +build !consulent

package proxycfgglue

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbsubscribe"
)

func (s serverIntentions) buildSubjects(serviceName string, entMeta acl.EnterpriseMeta) []*pbsubscribe.NamedSubject {
	// Based on getIntentionPrecedenceMatchServiceNames in the state package.
	if serviceName == structs.WildcardSpecifier {
		return []*pbsubscribe.NamedSubject{
			{
				Key:       structs.WildcardSpecifier,
				Namespace: entMeta.NamespaceOrDefault(),
				Partition: entMeta.PartitionOrDefault(),
				PeerName:  structs.DefaultPeerKeyword,
			},
		}
	}

	return []*pbsubscribe.NamedSubject{
		{
			Key:       serviceName,
			Namespace: entMeta.NamespaceOrDefault(),
			Partition: entMeta.PartitionOrDefault(),
			PeerName:  structs.DefaultPeerKeyword,
		},
		{
			Key:       structs.WildcardSpecifier,
			Namespace: entMeta.NamespaceOrDefault(),
			Partition: entMeta.PartitionOrDefault(),
			PeerName:  structs.DefaultPeerKeyword,
		},
	}
}
