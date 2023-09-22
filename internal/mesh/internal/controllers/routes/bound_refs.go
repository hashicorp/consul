// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package routes

import (
	"sort"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type sectionRefKey struct {
	resource.ReferenceKey
	Section string
}

type BoundReferenceCollector struct {
	refs map[sectionRefKey]*pbresource.Reference
}

func NewBoundReferenceCollector() *BoundReferenceCollector {
	return &BoundReferenceCollector{
		refs: make(map[sectionRefKey]*pbresource.Reference),
	}
}

func (c *BoundReferenceCollector) List() []*pbresource.Reference {
	if len(c.refs) == 0 {
		return nil
	}

	out := make([]*pbresource.Reference, 0, len(c.refs))
	for _, ref := range c.refs {
		out = append(out, ref)
	}

	sort.Slice(out, func(i, j int) bool {
		return resource.LessReference(out[i], out[j])
	})

	return out
}

func (c *BoundReferenceCollector) AddRefOrID(ref resource.ReferenceOrID) {
	if c == nil {
		return
	}
	c.AddRef(resource.ReferenceFromReferenceOrID(ref))
}

func (c *BoundReferenceCollector) AddRef(ref *pbresource.Reference) {
	if c == nil {
		return
	}
	srk := sectionRefKey{
		ReferenceKey: resource.NewReferenceKey(ref),
		Section:      ref.Section,
	}

	if _, ok := c.refs[srk]; ok {
		return
	}

	c.refs[srk] = ref
}
