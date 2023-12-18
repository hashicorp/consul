// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package index

import "github.com/hashicorp/consul/proto-public/pbresource"

//go:generate mockery --name resourceIterable --with-expecter --exported
type resourceIterable interface {
	Next() ([]byte, []*pbresource.Resource, bool)
}

type resourceIterator struct {
	current []*pbresource.Resource
	iter    resourceIterable
}

func (i *resourceIterator) Next() *pbresource.Resource {
	// Maybe get a new list from the internal iterator
	if len(i.current) == 0 {
		_, i.current, _ = i.iter.Next()
	}

	var rsc *pbresource.Resource
	switch len(i.current) {
	case 0:
		// we are completely out of data so we can return
	case 1:
		rsc = i.current[0]
		i.current = nil
	default:
		rsc = i.current[0]
		i.current = i.current[1:]
	}

	return rsc
}
