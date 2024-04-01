// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import "github.com/hashicorp/consul/api"

func PartitionOrDefault(name string) string {
	if name == "" {
		return "default"
	}
	return name
}
func NamespaceOrDefault(name string) string {
	if name == "" {
		return "default"
	}
	return name
}

func DefaultToEmpty(name string) string {
	if name == "default" {
		return ""
	}
	return name
}

// CompatQueryOpts cleans a QueryOptions so that Partition and Namespace fields
// are compatible with CE or ENT
// TODO: not sure why we can't do this server-side
func CompatQueryOpts(opts *api.QueryOptions) *api.QueryOptions {
	opts.Partition = DefaultToEmpty(opts.Partition)
	opts.Namespace = DefaultToEmpty(opts.Namespace)
	return opts
}
