// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package pbpeering

import "github.com/mitchellh/hashstructure"

func (p *Peering) GetHash() (uint64, error) {
	hash, err := hashstructure.Hash(p, nil)
	if err != nil {
		return 0, err
	}
	return hash, nil
}
