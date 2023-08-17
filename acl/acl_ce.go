// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package acl

const (
	WildcardPartitionName = ""
	DefaultPartitionName  = ""
)

// Reviewer Note: This is a little bit strange; one might want it to be "" like partition name
// However in consul/structs/intention.go we define IntentionDefaultNamespace as 'default' and so
// we use the same here
const DefaultNamespaceName = "default"

type EnterpriseConfig struct {
	// no fields in OSS
}

func (_ *EnterpriseConfig) Close() {
	// do nothing
}
