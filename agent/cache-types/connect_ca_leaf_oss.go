// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package cachetype

func (req *ConnectCALeafRequest) TargetNamespace() string {
	return "default"
}
