//go:build !consulent
// +build !consulent

package state

import "github.com/hashicorp/consul/acl"

func partitionedIndexEntryName(entry string, _ string) string {
	return entry
}

func partitionedAndNamespacedIndexEntryName(entry string, _ *acl.EnterpriseMeta) string {
	return entry
}
