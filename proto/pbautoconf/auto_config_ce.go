// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package pbautoconf

func (req *AutoConfigRequest) PartitionOrDefault() string {
	return ""
}
