// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package builder

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"

	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/mesh/internal/types/intermediate"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	"github.com/hashicorp/consul/internal/testing/golden"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestBuildMultiportImplicitDestinations(t *testing.T) {
	// TODO(rb/v2): add a fetchertest package to construct implicit upstreams
	// correctly from inputs. the following is far too manual and error prone
	// to be an accurate representation of what implicit upstreams look like.
	const (
		apiApp      = "api-app"
		apiApp2     = "api-app2"
		trustDomain = "foo.consul"
		datacenter  = "dc1"
	)
	proxyCfg := &pbmesh.ProxyConfiguration{
		DynamicConfig: &pbmesh.DynamicConfig{
			Mode: pbmesh.ProxyMode_PROXY_MODE_TRANSPARENT,
			TransparentProxy: &pbmesh.TransparentProxy{
				OutboundListenerPort: 15001,
			},
		},
	}

	multiportEndpointsData := &pbcatalog.ServiceEndpoints{
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "10.0.0.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
	}
	apiAppService := resourcetest.Resource(pbcatalog.ServiceType, apiApp).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	apiApp2Service := resourcetest.Resource(pbcatalog.ServiceType, apiApp2).
		WithTenancy(resource.DefaultNamespacedTenancy()).
		WithData(t, serviceData).
		Build()

	apiAppEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, apiApp).
		WithOwner(apiAppService.Id).
		WithData(t, multiportEndpointsData).
		WithTenancy(resource.DefaultNamespacedTenancy()).Build()

	apiApp2Endpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, apiApp2).
		WithOwner(apiApp2Service.Id).
		WithData(t, multiportEndpointsData).
		WithTenancy(resource.DefaultNamespacedTenancy()).Build()

	mwEndpointsData := &pbcatalog.ServiceEndpoints{ // variant on apiAppEndpoints
		Endpoints: []*pbcatalog.Endpoint{
			{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "10.0.0.1"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
			{
				Addresses: []*pbcatalog.WorkloadAddress{
					{Host: "10.0.0.2"},
				},
				Ports: map[string]*pbcatalog.WorkloadPort{
					"admin-port": {Port: 8080, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"api-port":   {Port: 9090, Protocol: pbcatalog.Protocol_PROTOCOL_TCP},
					"mesh":       {Port: 20000, Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
				},
			},
		},
	}
	mwEndpoints := resourcetest.Resource(pbcatalog.ServiceEndpointsType, apiApp).
		WithOwner(apiAppService.Id).
		WithData(t, mwEndpointsData).
		WithTenancy(resource.DefaultNamespacedTenancy()).Build()

	apiAppIdentity := &pbresource.Reference{
		Name:    fmt.Sprintf("%s-identity", apiApp),
		Tenancy: apiAppEndpoints.Id.Tenancy,
	}

	apiApp2Identity := &pbresource.Reference{
		Name:    fmt.Sprintf("%s-identity", apiApp2),
		Tenancy: apiApp2Endpoints.Id.Tenancy,
	}

	apiAppComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, apiAppService.Id)
	apiAppComputedRoutes := routestest.BuildComputedRoutes(t, apiAppComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, apiAppService),
	)
	require.NotNil(t, apiAppComputedRoutes)

	apiApp2ComputedRoutesID := resource.ReplaceType(pbmesh.ComputedRoutesType, apiApp2Service.Id)
	apiApp2ComputedRoutes := routestest.BuildComputedRoutes(t, apiApp2ComputedRoutesID,
		resourcetest.MustDecode[*pbcatalog.Service](t, apiApp2Service),
	)
	require.NotNil(t, apiApp2ComputedRoutes)

	newImplicitDestination := func(
		svc *pbresource.Resource,
		endpoints *pbresource.Resource,
		computedRoutes *types.DecodedComputedRoutes,
		identities []*pbresource.Reference,
		virtualIPs []string,
	) []*intermediate.Destination {
		svcDec := resourcetest.MustDecode[*pbcatalog.Service](t, svc)
		seDec := resourcetest.MustDecode[*pbcatalog.ServiceEndpoints](t, endpoints)

		var out []*intermediate.Destination
		for _, port := range svcDec.Data.Ports {
			portName := port.TargetPort
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				continue
			}

			dest := &intermediate.Destination{
				Service: svcDec,
				ComputedPortRoutes: routestest.MutateTargets(t, computedRoutes.Data, portName, func(t *testing.T, details *pbmesh.BackendTargetDetails) {
					switch {
					case resource.ReferenceOrIDMatch(svc.Id, details.BackendRef.Ref) && details.BackendRef.Port == portName:
						details.ServiceEndpointsId = endpoints.Id
						details.ServiceEndpoints = seDec.Data
						details.IdentityRefs = identities
					}
				}),
				VirtualIPs: virtualIPs,
			}
			out = append(out, dest)
		}
		return out
	}

	apiAppDestinations := newImplicitDestination(
		apiAppService,
		apiAppEndpoints,
		apiAppComputedRoutes,
		[]*pbresource.Reference{apiAppIdentity},
		[]string{"1.1.1.1"},
	)

	apiApp2Destinations := newImplicitDestination(
		apiApp2Service,
		apiApp2Endpoints,
		apiApp2ComputedRoutes,
		[]*pbresource.Reference{apiApp2Identity},
		[]string{"2.2.2.2", "3.3.3.3"},
	)

	mwDestinations := newImplicitDestination(
		apiAppService,
		mwEndpoints,
		apiAppComputedRoutes,
		[]*pbresource.Reference{apiAppIdentity},
		[]string{"1.1.1.1"},
	)

	twoImplicitDestinations := append(
		append([]*intermediate.Destination{}, apiAppDestinations...),
		apiApp2Destinations...,
	)

	cases := map[string]struct {
		getDestinations func() []*intermediate.Destination
	}{
		// Most basic test that multiport configuration works
		"destination/multiport-l4-single-implicit-destination-tproxy": {
			getDestinations: func() []*intermediate.Destination { return apiAppDestinations },
		},
		// Test shows that with multiple workloads for a service exposing the same ports, the routers
		// and clusters do not get duplicated.
		"destination/multiport-l4-single-implicit-destination-with-multiple-workloads-tproxy": {
			getDestinations: func() []*intermediate.Destination { return mwDestinations },
		},
		// Test shows that with multiple workloads for a service exposing the same ports, the routers
		// and clusters do not get duplicated.
		"destination/multiport-l4-multiple-implicit-destinations-tproxy": {
			getDestinations: func() []*intermediate.Destination { return twoImplicitDestinations },
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			proxyTmpl := New(testProxyStateTemplateID(), testIdentityRef(), trustDomain, datacenter, false, proxyCfg).
				BuildDestinations(c.getDestinations()).
				Build()

			// sort routers because of test flakes where order was flip flopping.
			actualRouters := proxyTmpl.ProxyState.Listeners[0].Routers
			sort.Slice(actualRouters, func(i, j int) bool {
				return actualRouters[i].String() < actualRouters[j].String()
			})

			actual := protoToJSON(t, proxyTmpl)
			expected := JSONToProxyTemplate(t, golden.GetBytes(t, actual, name+".golden"))

			// sort routers on listener from golden file
			expectedRouters := expected.ProxyState.Listeners[0].Routers
			sort.Slice(expectedRouters, func(i, j int) bool {
				return expectedRouters[i].String() < expectedRouters[j].String()
			})

			// convert back to json after sorting so that test output does not contain extraneous fields.
			require.Equal(t, protoToJSON(t, expected), protoToJSON(t, proxyTmpl))
		})
	}
}
