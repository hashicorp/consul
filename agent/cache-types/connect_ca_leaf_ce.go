// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package cachetype

func (req *ConnectCALeafRequest) TargetNamespace() string {
	return "default"
}
