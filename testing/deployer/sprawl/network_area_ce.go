// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package sprawl

func (s *Sprawl) initNetworkAreas() error {
	return nil
}

func (s *Sprawl) waitForNetworkAreaEstablishment() error {
	return nil
}
