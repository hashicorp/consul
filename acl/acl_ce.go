// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package acl

const (
	WildcardPartitionName = ""
	DefaultPartitionName  = ""
	// NonEmptyDefaultPartitionName is the name of the default partition that is
	// not empty.  An example of this being supplied is when a partition is specified
	// in the request for DNS by consul-dataplane.  This has been added to support
	// DNS v1.5, which needs to be compatible with the original DNS subsystem which
	// supports partition being "default" or empty.  Otherwise, use DefaultPartitionName.
	NonEmptyDefaultPartitionName = "default"

	// DefaultNamespaceName is used to mimic the behavior in consul/structs/intention.go,
	// where we define IntentionDefaultNamespace as 'default' and so we use the same here.
	// This is a little bit strange; one might want it to be "" like DefaultPartitionName.
	DefaultNamespaceName = "default"

	// EmptyNamespaceName is the name of the default partition that is an empty string.
	// An example of this being supplied is when a namespace is specifiedDNS v1.
	// EmptyNamespaceName has been added to support DNS v1.5, which needs to be
	// compatible with the original DNS subsystem which supports partition being "default" or empty.
	// Otherwise, use DefaultNamespaceName.
	EmptyNamespaceName = ""
)

type EnterpriseConfig struct {
	// no fields in CE
}

func (_ *EnterpriseConfig) Close() {
	// do nothing
}
