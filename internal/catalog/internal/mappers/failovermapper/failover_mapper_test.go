// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failovermapper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestMapper_Tracking(t *testing.T) {
	registry := resource.NewRegistry()
	types.Register(registry)

	// Create an advance pointer to some services.
	randoSvc := rtest.Resource(pbcatalog.ServiceType, "rando").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Service{}).
		Build()
	rtest.ValidateAndNormalize(t, registry, randoSvc)

	apiSvc := rtest.Resource(pbcatalog.ServiceType, "api").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Service{}).
		Build()
	rtest.ValidateAndNormalize(t, registry, apiSvc)

	fooSvc := rtest.Resource(pbcatalog.ServiceType, "foo").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Service{}).
		Build()
	rtest.ValidateAndNormalize(t, registry, fooSvc)

	barSvc := rtest.Resource(pbcatalog.ServiceType, "bar").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Service{}).
		Build()
	rtest.ValidateAndNormalize(t, registry, barSvc)

	wwwSvc := rtest.Resource(pbcatalog.ServiceType, "www").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.Service{}).
		Build()
	rtest.ValidateAndNormalize(t, registry, wwwSvc)

	fail1 := rtest.Resource(pbcatalog.FailoverPolicyType, "api").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.FailoverPolicy{
			Config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.ServiceType, "foo")},
					{Ref: newRef(pbcatalog.ServiceType, "bar")},
				},
			},
		}).
		Build()
	rtest.ValidateAndNormalize(t, registry, fail1)
	failDec1 := rtest.MustDecode[*pbcatalog.FailoverPolicy](t, fail1)

	fail2 := rtest.Resource(pbcatalog.FailoverPolicyType, "www").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.FailoverPolicy{
			Config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.ServiceType, "www"), Datacenter: "dc2"},
					{Ref: newRef(pbcatalog.ServiceType, "foo")},
				},
			},
		}).
		Build()
	rtest.ValidateAndNormalize(t, registry, fail2)
	failDec2 := rtest.MustDecode[*pbcatalog.FailoverPolicy](t, fail2)

	fail1_updated := rtest.Resource(pbcatalog.FailoverPolicyType, "api").
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, &pbcatalog.FailoverPolicy{
			Config: &pbcatalog.FailoverConfig{
				Destinations: []*pbcatalog.FailoverDestination{
					{Ref: newRef(pbcatalog.ServiceType, "bar")},
				},
			},
		}).
		Build()
	rtest.ValidateAndNormalize(t, registry, fail1_updated)
	failDec1_updated := rtest.MustDecode[*pbcatalog.FailoverPolicy](t, fail1_updated)

	m := New()

	// Nothing tracked yet so we assume nothing.
	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc)

	// no-ops
	m.UntrackFailover(fail1.Id)

	// still nothing
	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc)

	// Actually insert some data.
	m.TrackFailover(failDec1)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail1.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc)

	// track it again, no change
	m.TrackFailover(failDec1)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail1.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc)

	// track new one that overlaps slightly
	m.TrackFailover(failDec2)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail1.Id, fail2.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// update the original to change it
	m.TrackFailover(failDec1_updated)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc, fail1.Id)
	requireServicesTracked(t, m, fooSvc, fail2.Id)
	requireServicesTracked(t, m, barSvc, fail1.Id)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// delete the original
	m.UntrackFailover(fail1.Id)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc, fail2.Id)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc, fail2.Id)

	// delete the other one
	m.UntrackFailover(fail2.Id)

	requireServicesTracked(t, m, randoSvc)
	requireServicesTracked(t, m, apiSvc)
	requireServicesTracked(t, m, fooSvc)
	requireServicesTracked(t, m, barSvc)
	requireServicesTracked(t, m, wwwSvc)
}

func requireServicesTracked(t *testing.T, mapper *Mapper, svc *pbresource.Resource, failovers ...*pbresource.ID) {
	t.Helper()

	reqs, err := mapper.MapService(
		context.Background(),
		controller.Runtime{},
		svc,
	)
	require.NoError(t, err)

	require.Len(t, reqs, len(failovers))

	for _, failover := range failovers {
		prototest.AssertContainsElement(t, reqs, controller.Request{ID: failover})
	}
}

func newRef(typ *pbresource.Type, name string) *pbresource.Reference {
	return rtest.Resource(typ, name).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		Reference("")
}
