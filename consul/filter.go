package consul

import (
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/consul/structs"
)

func FilterDirEnt(acl acl.ACL, ent structs.DirEntries) structs.DirEntries {
	// Remove any keys blocked by ACLs
	removed := 0
	for i := 0; i < len(ent); i++ {
		if !acl.KeyRead(ent[i].Key) {
			ent[i] = nil
			removed++
		}
	}

	// Compact the list
	dst := 0
	src := 0
	n := len(ent) - removed
	for dst < n {
		for ent[src] == nil && src < n {
			src++
		}
		end := src + 1
		for ent[end] != nil && end < n {
			end++
		}
		span := end - src
		copy(ent[dst:dst+span], ent[src:src+span])
		dst += span
		src += span
	}

	// Trim the entries
	ent = ent[:n]
	return ent
}
