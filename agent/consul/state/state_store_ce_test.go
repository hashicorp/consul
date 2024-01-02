// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package state

func (s *Store) setupDefaultTestEntMeta() error {
	return nil
}
