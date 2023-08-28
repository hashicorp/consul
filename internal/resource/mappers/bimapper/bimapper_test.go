// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bimapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

const (
	fakeGroupName = "catalog"
	fakeVersion   = "v1"
)

var (
	fakeFooType = &pbresource.Type{
		Group:        fakeGroupName,
		GroupVersion: fakeVersion,
		Kind:         "Foo",
	}
	fakeBarType = &pbresource.Type{
		Group:        fakeGroupName,
		GroupVersion: fakeVersion,
		Kind:         "Bar",
	}
)

func testRegistry() resource.Registry {
	registry := resource.NewRegistry()
	registry.Register(resource.Registration{
		Type: fakeFooType,
		// Ignore values for required fields proto and scope.
		Proto: &pbresource.ID{},
		Scope: resource.ScopeNamespace,
	})
	registry.Register(resource.Registration{
		Type: fakeBarType,
		// Ignore values for required fields proto and scope.
		Proto: &pbresource.ID{},
		Scope: resource.ScopeNamespace,
	})

	return registry
}

func TestMapper(t *testing.T) {
	// Create an advance pointer to some services.
	registry := testRegistry()

	randoSvc := rtest.Resource(fakeBarType, "rando", registry).Build()
	apiSvc := rtest.Resource(fakeBarType, "api", registry).Build()
	fooSvc := rtest.Resource(fakeBarType, "foo", registry).Build()
	barSvc := rtest.Resource(fakeBarType, "bar", registry).Build()
	wwwSvc := rtest.Resource(fakeBarType, "www", registry).Build()

	apiRef := newRef(fakeBarType, "api", registry)
	fooRef := newRef(fakeBarType, "foo", registry)
	barRef := newRef(fakeBarType, "bar", registry)
	wwwRef := newRef(fakeBarType, "www", registry)

	fail1 := rtest.Resource(fakeFooType, "api", registry).Build()
	fail1Refs := []resource.ReferenceOrID{
		apiRef,
		fooRef,
		barRef,
	}

	fail2 := rtest.Resource(fakeFooType, "www", registry).Build()
	fail2Refs := []resource.ReferenceOrID{
		wwwRef,
		fooRef,
	}

	fail1UpdatedRefs := []resource.ReferenceOrID{
		apiRef,
		barRef,
	}

	m := New(fakeFooType, fakeBarType)

	// Nothing tracked yet so we assume nothing.
	requireLinksForItem(t, m, fail1.Id)
	requireLinksForItem(t, m, fail2.Id)
	requireItemsForLink(t, m, apiRef)
	requireItemsForLink(t, m, fooRef)
	requireItemsForLink(t, m, barRef)
	requireItemsForLink(t, m, wwwRef)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc)

	// no-ops
	m.UntrackItem(fail1.Id)

	// still nothing
	requireLinksForItem(t, m, fail1.Id)
	requireLinksForItem(t, m, fail2.Id)
	requireItemsForLink(t, m, apiRef)
	requireItemsForLink(t, m, fooRef)
	requireItemsForLink(t, m, barRef)
	requireItemsForLink(t, m, wwwRef)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc)

	// Actually insert some data.
	m.TrackItem(fail1.Id, fail1Refs)

	// Check links mapping
	requireLinksForItem(t, m, fail1.Id, fail1Refs...)

	requireLinksForItem(t, m, fail1.Id, fail1Refs...)
	requireItemsForLink(t, m, apiRef, fail1.Id)
	requireItemsForLink(t, m, fooRef, fail1.Id)
	requireItemsForLink(t, m, barRef, fail1.Id)
	requireItemsForLink(t, m, wwwRef)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail1.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc)

	// track it again, no change
	m.TrackItem(fail1.Id, fail1Refs)

	requireLinksForItem(t, m, fail1.Id, fail1Refs...)
	requireItemsForLink(t, m, apiRef, fail1.Id)
	requireItemsForLink(t, m, fooRef, fail1.Id)
	requireItemsForLink(t, m, barRef, fail1.Id)
	requireItemsForLink(t, m, wwwRef)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail1.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc)

	// track new one that overlaps slightly
	m.TrackItem(fail2.Id, fail2Refs)

	// Check links mapping for the new one
	requireLinksForItem(t, m, fail1.Id, fail1Refs...)
	requireLinksForItem(t, m, fail2.Id, fail2Refs...)
	requireItemsForLink(t, m, apiRef, fail1.Id)
	requireItemsForLink(t, m, fooRef, fail1.Id, fail2.Id)
	requireItemsForLink(t, m, barRef, fail1.Id)
	requireItemsForLink(t, m, wwwRef, fail2.Id)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail1.Id, fail2.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// update the original to change it
	m.TrackItem(fail1.Id, fail1UpdatedRefs)

	requireLinksForItem(t, m, fail1.Id, fail1UpdatedRefs...)
	requireLinksForItem(t, m, fail2.Id, fail2Refs...)
	requireItemsForLink(t, m, apiRef, fail1.Id)
	requireItemsForLink(t, m, fooRef, fail2.Id)
	requireItemsForLink(t, m, barRef, fail1.Id)
	requireItemsForLink(t, m, wwwRef, fail2.Id)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail2.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// delete the original
	m.UntrackItem(fail1.Id)

	requireLinksForItem(t, m, fail1.Id)
	requireLinksForItem(t, m, fail2.Id, fail2Refs...)
	requireItemsForLink(t, m, apiRef)
	requireItemsForLink(t, m, fooRef, fail2.Id)
	requireItemsForLink(t, m, barRef)
	requireItemsForLink(t, m, wwwRef, fail2.Id)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc, fail2.Id)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// delete the link
	m.UntrackLink(newRef(fakeBarType, "www", registry))

	requireLinksForItem(t, m, fail2.Id, newRef(fakeBarType, "foo", registry))

	m.UntrackLink(newRef(fakeBarType, "foo", registry))

	requireLinksForItem(t, m, fail2.Id)

	// delete another item
	m.UntrackItem(fail2.Id)

	requireLinksForItem(t, m, fail1.Id)
	requireLinksForItem(t, m, fail2.Id)
	requireItemsForLink(t, m, apiRef)
	requireItemsForLink(t, m, fooRef)
	requireItemsForLink(t, m, barRef)
	requireItemsForLink(t, m, wwwRef)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc)

	// Reset the mapper and check that its internal maps are empty.
	m.Reset()
	require.True(t, m.IsEmpty())
}

func TestPanics(t *testing.T) {
	registry := testRegistry()

	t.Run("new mapper without types", func(t *testing.T) {
		require.PanicsWithValue(t, "itemType is required", func() {
			New(nil, nil)
		})

		require.PanicsWithValue(t, "itemType is required", func() {
			New(nil, fakeBarType)
		})

		require.PanicsWithValue(t, "linkType is required", func() {
			New(fakeFooType, nil)
		})
	})

	t.Run("UntrackItem: mismatched type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected item type \"catalog.v1.Foo\" got \"catalog.v1.Bar\"", func() {
			// Calling UntrackItem with link type instead of item type
			m.UntrackItem(rtest.Resource(fakeBarType, "test", registry).ID())
		})
	})

	t.Run("TrackItem: mismatched item type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected item type \"catalog.v1.Foo\" got \"catalog.v1.Bar\"", func() {
			// Calling UntrackItem with link type instead of item type
			m.TrackItem(rtest.Resource(fakeBarType, "test", registry).ID(), nil)
		})
	})

	t.Run("TrackItem: mismatched link type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected link type \"catalog.v1.Bar\" got \"catalog.v1.Foo\"", func() {
			// Calling UntrackItem with link type instead of item type
			links := []resource.ReferenceOrID{
				rtest.Resource(fakeFooType, "link", registry).ID(),
			}
			m.TrackItem(rtest.Resource(fakeFooType, "test", registry).ID(), links)
		})
	})

	t.Run("UntrackLink: mismatched type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected link type \"catalog.v1.Bar\" got \"catalog.v1.Foo\"", func() {
			m.UntrackLink(rtest.Resource(fakeFooType, "test", registry).ID())
		})
	})

	t.Run("LinkRefsForItem: mismatched type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected item type \"catalog.v1.Foo\" got \"catalog.v1.Bar\"", func() {
			m.LinkRefsForItem(rtest.Resource(fakeBarType, "test", registry).ID())
		})
	})

	t.Run("LinkRefsForItem: mismatched type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected item type \"catalog.v1.Foo\" got \"catalog.v1.Bar\"", func() {
			m.LinkIDsForItem(rtest.Resource(fakeBarType, "test", registry).ID())
		})
	})

	t.Run("ItemRefsForLink: mismatched type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected link type \"catalog.v1.Bar\" got \"catalog.v1.Foo\"", func() {
			m.ItemRefsForLink(rtest.Resource(fakeFooType, "test", registry).ID())
		})
	})

	t.Run("ItemIDsForLink: mismatched type", func(t *testing.T) {
		m := New(fakeFooType, fakeBarType)
		require.PanicsWithValue(t, "expected link type \"catalog.v1.Bar\" got \"catalog.v1.Foo\"", func() {
			m.ItemIDsForLink(rtest.Resource(fakeFooType, "test", registry).ID())
		})
	})
}

func requireServicesTracked(t *testing.T, mapper *Mapper, link *pbresource.Resource, items ...*pbresource.ID) {
	t.Helper()

	reqs, err := mapper.MapLink(
		context.Background(),
		controller.Runtime{},
		link,
	)
	require.NoError(t, err)

	require.Len(t, reqs, len(items))

	// Also check items IDs and Refs for link.
	ids := mapper.ItemIDsForLink(link.Id)
	require.Len(t, ids, len(items))

	refs := mapper.ItemRefsForLink(link.Id)
	require.Len(t, refs, len(items))

	for _, item := range items {
		prototest.AssertContainsElement(t, reqs, controller.Request{ID: item})
		prototest.AssertContainsElement(t, ids, item)
		prototest.AssertContainsElement(t, refs, resource.Reference(item, ""))
	}
}

func requireItemsForLink(t *testing.T, mapper *Mapper, link *pbresource.Reference, items ...*pbresource.ID) {
	t.Helper()

	got := mapper.ItemIDsForLink(resource.IDFromReference(link))

	prototest.AssertElementsMatch(t, items, got)
}

func requireLinksForItem(t *testing.T, mapper *Mapper, item *pbresource.ID, links ...resource.ReferenceOrID) {
	t.Helper()

	var expLinkRefs []*pbresource.Reference
	var expLinkIDs []*pbresource.ID

	for _, l := range links {
		expLinkRefs = append(expLinkRefs, &pbresource.Reference{
			Name:    l.GetName(),
			Tenancy: l.GetTenancy(),
			Type:    l.GetType(),
		})
		expLinkIDs = append(expLinkIDs, &pbresource.ID{
			Name:    l.GetName(),
			Tenancy: l.GetTenancy(),
			Type:    l.GetType(),
		})
	}

	refs := mapper.LinkRefsForItem(item)
	require.Len(t, refs, len(links))
	prototest.AssertElementsMatch(t, expLinkRefs, refs)

	ids := mapper.LinkIDsForItem(item)
	require.Len(t, refs, len(links))
	prototest.AssertElementsMatch(t, expLinkIDs, ids)
}

func newRef(typ *pbresource.Type, name string, registry resource.Registry) *pbresource.Reference {
	return rtest.Resource(typ, name, registry).Reference("")
}
