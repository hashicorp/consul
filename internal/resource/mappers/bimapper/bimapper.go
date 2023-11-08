// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bimapper

import (
	"context"
	"fmt"
	"sync"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var wildcardType = &pbresource.Type{
	Group:        "@@any@@",
	GroupVersion: "@@any@@",
	Kind:         "@@any@@",
}

// Mapper tracks bidirectional lookup for an item that contains references to
// other items. For example: an HTTPRoute has many references to Services.
//
// The primary object is called the "item" and an item has many "links".
// Tracking is done on items.
type Mapper struct {
	itemType, linkType *pbresource.Type
	wildcardLink       bool

	lock       sync.Mutex
	itemToLink map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}
	linkToItem map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}
}

// New creates a bimapper between the two required provided types.
func New(itemType, linkType *pbresource.Type) *Mapper {
	if itemType == nil {
		panic("itemType is required")
	}
	if linkType == nil {
		panic("linkType is required")
	}
	return &Mapper{
		itemType:   itemType,
		linkType:   linkType,
		itemToLink: make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}),
		linkToItem: make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{}),
	}
}

// NewWithWildcardLinkType creates a bimapper between the provided item type
// and can have a mixed set of link types.
func NewWithWildcardLinkType(itemType *pbresource.Type) *Mapper {
	m := New(itemType, wildcardType)
	m.wildcardLink = true
	return m
}

// Reset clears the internal mappings.
func (m *Mapper) Reset() {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.itemToLink = make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{})
	m.linkToItem = make(map[resource.ReferenceKey]map[resource.ReferenceKey]struct{})
}

// IsEmpty returns true if the internal structures are empty.
func (m *Mapper) IsEmpty() bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.itemToLink) == 0 && len(m.linkToItem) == 0
}

// UntrackItem removes tracking for the provided item. The item type MUST match
// the type configured for the item.
func (m *Mapper) UntrackItem(item resource.ReferenceOrID) {
	if !resource.EqualType(item.GetType(), m.itemType) {
		panic(fmt.Sprintf("expected item type %q got %q",
			resource.TypeToString(m.itemType),
			resource.TypeToString(item.GetType()),
		))
	}
	m.untrackItem(resource.NewReferenceKey(item))
}

// UntrackLink removes tracking for the provided link. The link type MUST match
// the type configured for the link.
func (m *Mapper) UntrackLink(link resource.ReferenceOrID) {
	if !m.wildcardLink && !resource.EqualType(link.GetType(), m.linkType) {
		panic(fmt.Sprintf("expected link type %q got %q",
			resource.TypeToString(m.linkType),
			resource.TypeToString(link.GetType()),
		))
	}
	m.untrackLink(resource.NewReferenceKey(link))
}

func (m *Mapper) untrackLink(link resource.ReferenceKey) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removeLinkLocked(link)
}

func (m *Mapper) untrackItem(item resource.ReferenceKey) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removeItemLocked(item)
}

// TrackItem adds tracking for the provided item. The item and link types MUST
// match the types configured for the items and links.
func (m *Mapper) TrackItem(item resource.ReferenceOrID, links []resource.ReferenceOrID) {
	if !resource.EqualType(item.GetType(), m.itemType) {
		panic(fmt.Sprintf("expected item type %q got %q",
			resource.TypeToString(m.itemType),
			resource.TypeToString(item.GetType()),
		))
	}

	linksAsKeys := make([]resource.ReferenceKey, 0, len(links))
	for _, link := range links {
		if !m.wildcardLink && !resource.EqualType(link.GetType(), m.linkType) {
			panic(fmt.Sprintf("expected link type %q got %q",
				resource.TypeToString(m.linkType),
				resource.TypeToString(link.GetType()),
			))
		}
		linksAsKeys = append(linksAsKeys, resource.NewReferenceKey(link))
	}

	m.trackItem(resource.NewReferenceKey(item), linksAsKeys)
}

func (m *Mapper) trackItem(item resource.ReferenceKey, links []resource.ReferenceKey) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.removeItemLocked(item)
	m.addItemLocked(item, links)
}

// you must hold the lock before calling this function
func (m *Mapper) removeItemLocked(item resource.ReferenceKey) {
	for link := range m.itemToLink[item] {
		delete(m.linkToItem[link], item)
		if len(m.linkToItem[link]) == 0 {
			delete(m.linkToItem, link)
		}
	}
	delete(m.itemToLink, item)
}

func (m *Mapper) removeLinkLocked(link resource.ReferenceKey) {
	for item := range m.linkToItem[link] {
		delete(m.itemToLink[item], link)
		if len(m.itemToLink[item]) == 0 {
			delete(m.itemToLink, item)
		}
	}
	delete(m.linkToItem, link)
}

// you must hold the lock before calling this function
func (m *Mapper) addItemLocked(item resource.ReferenceKey, links []resource.ReferenceKey) {
	if m.itemToLink[item] == nil {
		m.itemToLink[item] = make(map[resource.ReferenceKey]struct{})
	}
	for _, link := range links {
		m.itemToLink[item][link] = struct{}{}

		if m.linkToItem[link] == nil {
			m.linkToItem[link] = make(map[resource.ReferenceKey]struct{})
		}
		m.linkToItem[link][item] = struct{}{}
	}
}

// LinksForItem returns references to links related to the requested item.
// Deprecated: use LinksRefs
func (m *Mapper) LinksForItem(item *pbresource.ID) []*pbresource.Reference {
	return m.LinkRefsForItem(item)
}

// LinkRefsForItem returns references to links related to the requested item.
func (m *Mapper) LinkRefsForItem(item *pbresource.ID) []*pbresource.Reference {
	if !resource.EqualType(item.Type, m.itemType) {
		panic(fmt.Sprintf("expected item type %q got %q",
			resource.TypeToString(m.itemType),
			resource.TypeToString(item.Type),
		))
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	links, ok := m.itemToLink[resource.NewReferenceKey(item)]
	if !ok {
		return nil
	}

	out := make([]*pbresource.Reference, 0, len(links))
	for link := range links {
		out = append(out, link.ToReference())
	}
	return out
}

// LinkIDsForItem returns IDs to links related to the requested item.
func (m *Mapper) LinkIDsForItem(item *pbresource.ID) []*pbresource.ID {
	if !resource.EqualType(item.Type, m.itemType) {
		panic(fmt.Sprintf("expected item type %q got %q",
			resource.TypeToString(m.itemType),
			resource.TypeToString(item.Type),
		))
	}

	m.lock.Lock()
	defer m.lock.Unlock()

	links, ok := m.itemToLink[resource.NewReferenceKey(item)]
	if !ok {
		return nil
	}

	out := make([]*pbresource.ID, 0, len(links))
	for l := range links {
		out = append(out, l.ToID())
	}
	return out
}

// ItemsForLink returns item ids for items related to the provided link.
// Deprecated: use ItemIDsForLink
func (m *Mapper) ItemsForLink(link *pbresource.ID) []*pbresource.ID {
	return m.ItemIDsForLink(link)
}

// ItemIDsForLink returns item ids for items related to the provided link.
func (m *Mapper) ItemIDsForLink(link resource.ReferenceOrID) []*pbresource.ID {
	if !m.wildcardLink && !resource.EqualType(link.GetType(), m.linkType) {
		panic(fmt.Sprintf("expected link type %q got %q",
			resource.TypeToString(m.linkType),
			resource.TypeToString(link.GetType()),
		))
	}

	return m.itemIDsByLink(resource.NewReferenceKey(link))
}

// ItemRefsForLink returns item references for items related to the provided link.
func (m *Mapper) ItemRefsForLink(link resource.ReferenceOrID) []*pbresource.Reference {
	if !m.wildcardLink && !resource.EqualType(link.GetType(), m.linkType) {
		panic(fmt.Sprintf("expected link type %q got %q",
			resource.TypeToString(m.linkType),
			resource.TypeToString(link.GetType()),
		))
	}

	return m.itemRefsByLink(resource.NewReferenceKey(link))
}

// MapLink is suitable as a DependencyMapper to map the provided link event to its item.
func (m *Mapper) MapLink(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	link := res.Id

	if !m.wildcardLink && !resource.EqualType(link.Type, m.linkType) {
		return nil, fmt.Errorf("expected type %q got %q",
			resource.TypeToString(m.linkType),
			resource.TypeToString(link.Type),
		)
	}

	itemIDs := m.itemIDsByLink(resource.NewReferenceKey(link))

	out := make([]controller.Request, 0, len(itemIDs))
	for _, item := range itemIDs {
		if !resource.EqualType(item.Type, m.itemType) {
			return nil, fmt.Errorf("expected type %q got %q",
				resource.TypeToString(m.itemType),
				resource.TypeToString(item.Type),
			)
		}
		out = append(out, controller.Request{ID: item})
	}
	return out, nil
}

func (m *Mapper) itemIDsByLink(link resource.ReferenceKey) []*pbresource.ID {
	// a lock must be held both to read item from the map and to read the
	// the returned items.
	m.lock.Lock()
	defer m.lock.Unlock()

	items, ok := m.linkToItem[link]
	if !ok {
		return nil
	}

	out := make([]*pbresource.ID, 0, len(items))
	for item := range items {
		out = append(out, item.ToID())
	}
	return out
}

func (m *Mapper) itemRefsByLink(link resource.ReferenceKey) []*pbresource.Reference {
	// a lock must be held both to read item from the map and to read the
	// the returned items.
	m.lock.Lock()
	defer m.lock.Unlock()

	items, ok := m.linkToItem[link]
	if !ok {
		return nil
	}

	out := make([]*pbresource.Reference, 0, len(items))
	for item := range items {
		out = append(out, item.ToReference())
	}
	return out
}
