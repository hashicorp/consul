// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package types

type ExpandedConsumers struct {
	Peers                 []string
	Partitions            []string
	MissingSamenessGroups []string
}
