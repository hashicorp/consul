// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package structs

func (e *MeshConfigEntry) validateEnterpriseMeta() error {
	return nil
}
