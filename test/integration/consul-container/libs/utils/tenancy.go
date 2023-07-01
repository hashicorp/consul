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

// PartitionQueryOptions returns an *api.QueryOptions with the given partition
// field set only if the partition is non-default. This helps when writing
// tests for joint use in OSS and ENT.
func PartitionQueryOptions(partition string) *api.QueryOptions {
	return &api.QueryOptions{
		Partition: DefaultToEmpty(partition),
	}
}
