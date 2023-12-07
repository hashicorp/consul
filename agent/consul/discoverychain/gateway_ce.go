//go:build !consulent

// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package discoverychain

// entTargetNamespace returns the formatted namespace
// for targeting a specific upstream in the Enterprise version of Consul.
// Its behavior is non-enterprise environments is to return an empty string.
func entTargetNamespace(ns string) string {
	return ""
}

// entTargetPartition returns the formatted partition
// for targeting a specific upstream in the Enterprise version of Consul.
// Its behavior is non-enterprise environments is to return an empty string.
func entTargetPartition(pt string) string {
	return ""
}
