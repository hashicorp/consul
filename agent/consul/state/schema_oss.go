//go:build !consulent
// +build !consulent

package state

import "github.com/hashicorp/consul/agent/structs"

func partitionedIndexEntryName(entry string, _ string) string {
	return entry
}

func partitionedAndNamespacedIndexEntryName(entry string, _ *structs.EnterpriseMeta) string {
	return entry
}
