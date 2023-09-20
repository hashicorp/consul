// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package state

func (s *Store) setupDefaultTestEntMeta() error {
	return nil
}
