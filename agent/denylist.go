// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"github.com/armon/go-radix"
)

// Denylist implements an HTTP endpoint denylist based on a list of endpoint
// prefixes which should be blocked.
type Denylist struct {
	tree *radix.Tree
}

// NewDenylist returns a denylist for the given list of prefixes.
func NewDenylist(prefixes []string) *Denylist {
	tree := radix.New()
	for _, prefix := range prefixes {
		tree.Insert(prefix, nil)
	}
	return &Denylist{tree}
}

// Block will return true if the given path is included among any of the
// blocked prefixes.
func (d *Denylist) Block(path string) bool {
	_, _, blocked := d.tree.LongestPrefix(path)
	return blocked
}
