// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package structs

func (e *MeshConfigEntry) validateEnterpriseMeta() error {
	return nil
}
