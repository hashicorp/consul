// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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

func TestMapper(t *testing.T) {
	// Create an advance pointer to some services.

	randoSvc := rtest.Resource(fakeBarType, "rando").Build()
	apiSvc := rtest.Resource(fakeBarType, "api").Build()
	fooSvc := rtest.Resource(fakeBarType, "foo").Build()
	barSvc := rtest.Resource(fakeBarType, "bar").Build()
	wwwSvc := rtest.Resource(fakeBarType, "www").Build()

	apiRef := newRef(fakeBarType, "api")
	fooRef := newRef(fakeBarType, "foo")
	barRef := newRef(fakeBarType, "bar")
	wwwRef := newRef(fakeBarType, "www")

	fail1 := rtest.Resource(fakeFooType, "api").Build()
	fail1_refs := []*pbresource.Reference{
		apiRef,
		fooRef,
		barRef,
	}

	fail2 := rtest.Resource(fakeFooType, "www").Build()
	fail2_refs := []*pbresource.Reference{
		wwwRef,
		fooRef,
	}

	fail1_updated := rtest.Resource(fakeFooType, "api").Build()
	fail1_updated_refs := []*pbresource.Reference{
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
	m.TrackItem(fail1.Id, fail1_refs)

	requireLinksForItem(t, m, fail1.Id, fail1_refs...)
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
	m.TrackItem(fail1.Id, fail1_refs)

	requireLinksForItem(t, m, fail1.Id, fail1_refs...)
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
	m.TrackItem(fail2.Id, fail2_refs)

	requireLinksForItem(t, m, fail1.Id, fail1_refs...)
	requireLinksForItem(t, m, fail2.Id, fail2_refs...)
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
	m.TrackItem(fail1_updated.Id, fail1_updated_refs)

	requireLinksForItem(t, m, fail1.Id, fail1_updated_refs...)
	requireLinksForItem(t, m, fail2.Id, fail2_refs...)
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
	requireLinksForItem(t, m, fail2.Id, fail2_refs...)
	requireItemsForLink(t, m, apiRef)
	requireItemsForLink(t, m, fooRef, fail2.Id)
	requireItemsForLink(t, m, barRef)
	requireItemsForLink(t, m, wwwRef, fail2.Id)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc, fail2.Id)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// delete the other one
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

	for _, item := range items {
		prototest.AssertContainsElement(t, reqs, controller.Request{ID: item})
	}
}

func requireLinksForItem(t *testing.T, mapper *Mapper, item *pbresource.ID, links ...*pbresource.Reference) {
	t.Helper()

	got := mapper.LinksForItem(item)

	prototest.AssertElementsMatch(t, links, got)
}

func requireItemsForLink(t *testing.T, mapper *Mapper, link *pbresource.Reference, items ...*pbresource.ID) {
	t.Helper()

	got := mapper.ItemsForLink(resource.IDFromReference(link))

	prototest.AssertElementsMatch(t, items, got)
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return rtest.Resource(typ, name).Reference("")
}

func newID(typ *pbresource.Type, name string) *pbresource.ID {
	return rtest.Resource(typ, name).ID()
}

func defaultTenancy() *pbresource.Tenancy {
	return &pbresource.Tenancy{
		Partition: "default",
		Namespace: "default",
		PeerName:  "local",
	}
}
