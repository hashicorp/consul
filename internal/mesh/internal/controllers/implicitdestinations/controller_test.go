// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package implicitdestinations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/auth"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/routes/routestest"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbauth "github.com/hashicorp/consul/proto-public/pbauth/v2beta1"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/version/versiontest"
)

type controllerSuite struct {
	suite.Suite

	ctx    context.Context
	client *rtest.Client
	rt     controller.Runtime

	ctl          *controller.TestController
	tenancies    []*pbresource.Tenancy
	isEnterprise bool

	// Variants of the above for the few tests that need to verify
	// default-allow mode.
	rtDefaultAllow  controller.Runtime
	ctlDefaultAllow *controller.TestController
}

func (suite *controllerSuite) SetupTest() {
	suite.isEnterprise = versiontest.IsEnterprise()
	suite.tenancies = resourcetest.TestTenancies()
	registerTenancies := resourcetest.TestTenancies()
	if suite.isEnterprise {
		registerTenancies = append(registerTenancies,
			rtest.Tenancy("wild.aaa"),
			rtest.Tenancy("wild.bbb"),
			rtest.Tenancy("default.fixed"),
		)
	}

	suite.ctx = testutil.TestContext(suite.T())
	client := svctest.NewResourceServiceBuilder().
		WithRegisterFns(types.Register, catalog.RegisterTypes, auth.RegisterTypes).
		WithTenancies(registerTenancies...).
		Run(suite.T())

	// The normal one we do most tests with is default-deny.
	suite.ctl = controller.NewTestController(Controller(false), client).
		WithLogger(testutil.Logger(suite.T()))
	suite.rt = suite.ctl.Runtime()
	suite.client = rtest.NewClient(suite.rt.Client)

	// Also create one for default-allow. (we pass the derived caching client
	// from the first TestController in here so we can get both sets of caches
	// to update in unison.
	suite.ctlDefaultAllow = controller.NewTestController(Controller(true), suite.client).
		WithLogger(testutil.Logger(suite.T()))
	suite.rtDefaultAllow = suite.ctlDefaultAllow.Runtime()
	// One true client.
	suite.client = rtest.NewClient(suite.rtDefaultAllow.Client)
}

func (suite *controllerSuite) requireCID(resource *pbresource.Resource, expected *pbmesh.ComputedImplicitDestinations) {
	suite.T().Helper()
	dec := rtest.MustDecode[*pbmesh.ComputedImplicitDestinations](suite.T(), resource)
	prototest.AssertDeepEqual(suite.T(), expected, dec.Data)
}

func (suite *controllerSuite) createWorkloadIdentities(names []string, tenancy *pbresource.Tenancy) []*pbresource.Resource {
	return createWorkloadIdentities(suite.T(), suite.client, names, tenancy)
}

func createWorkloadIdentities(
	t *testing.T,
	client *rtest.Client,
	names []string,
	tenancy *pbresource.Tenancy,
) []*pbresource.Resource {
	var rs []*pbresource.Resource
	for _, n := range names {
		r := rtest.Resource(pbauth.WorkloadIdentityType, n).
			WithTenancy(tenancy).
			Write(t, client)
		rs = append(rs, r)
	}
	return rs
}

// TODO: have the CTP controller export an in-mem reconcile function like the routestest package
// that would help with the jumble of mutate+validate of TP + assembly into CTP
func (suite *controllerSuite) createTrafficPermissions(
	names []string,
	defaults []string,
	tenancy *pbresource.Tenancy,
) {
	suite.T().Helper()
	var (
		destinationName string
		sources         []*pbauth.Source

		tpByDest = make(map[string][]*pbauth.TrafficPermissions)
	)

	for _, n := range names {
		switch n {
		case "d-wi1-s-wi2":
			destinationName = "wi1"
			sources = []*pbauth.Source{{
				IdentityName: "wi2",
			}}
		case "d-wi1-s-wi3":
			destinationName = "wi1"
			sources = []*pbauth.Source{{
				IdentityName: "wi3",
			}}
		case "d-wi2-s-wi1":
			destinationName = "wi2"
			sources = []*pbauth.Source{{
				IdentityName: "wi1",
			}}
		case "d-wi2-s-wi3":
			destinationName = "wi2"
			sources = []*pbauth.Source{{
				IdentityName: "wi3",
			}}
		case "d-wi4-s-wi5":
			destinationName = "wi4"
			sources = []*pbauth.Source{{
				IdentityName: "wi5",
			}}
		case "d-wi3-s-wild-name":
			destinationName = "wi3"
			sources = []*pbauth.Source{{
				IdentityName: "",
			}}
		default:
			suite.T().Fatalf("unknown type of workload identity template: %s", n)
		}

		// Write it just so we get the mutate+validate part
		tp0 := rtest.Resource(pbauth.TrafficPermissionsType, "ignore").
			WithData(suite.T(), &pbauth.TrafficPermissions{
				Destination: &pbauth.Destination{
					IdentityName: destinationName,
				},
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: sources,
				}},
			}).
			WithTenancy(tenancy).
			Build()

		decTP0 := rtest.MustDecode[*pbauth.TrafficPermissions](suite.T(), tp0)

		tpByDest[destinationName] = append(tpByDest[destinationName], decTP0.Data)
	}

	// Insert just enough to get the default one made for free.
	for _, n := range defaults {
		some := tpByDest[n]
		require.Empty(suite.T(), some)
		tpByDest[n] = nil
	}

	for destinationName, tpList := range tpByDest {
		id := rtest.Resource(pbauth.ComputedTrafficPermissionsType, destinationName).
			WithTenancy(tenancy).
			ID()

		ReconcileComputedTrafficPermissions(
			suite.T(),
			suite.client,
			id,
			tpList...,
		)
	}
}

type serviceFixture struct {
	*pbresource.Resource
	StatusUpdate func() *pbresource.Resource
}

func (suite *controllerSuite) createServices(names []string, tenancy *pbresource.Tenancy, withWIDs bool) []*serviceFixture {
	return createServices(suite.T(), suite.client, names, tenancy, withWIDs)
}

func createServices(
	t *testing.T,
	client *rtest.Client,
	names []string,
	tenancy *pbresource.Tenancy,
	withWIDs bool,
) []*serviceFixture {
	var rs []*serviceFixture
	for _, n := range names {
		var (
			workloads []string
			ids       []string
		)
		switch n {
		case "s1":
			workloads = []string{"w1"}
			ids = []string{"wi1"}
		case "s2":
			workloads = []string{"w2"}
			ids = []string{"wi2"}
		case "s3":
			workloads = []string{"w3"}
			ids = []string{"wi3"}
		case "s4":
			workloads = []string{"w4"}
			ids = []string{"wi4"}
		case "s5":
			workloads = []string{"w5"}
			ids = []string{"wi5"}
		case "s1-2":
			workloads = []string{"w1", "w2"}
			ids = []string{"wi1", "wi2"}
		case "s11-2":
			workloads = []string{"w1-1", "w2"}
			ids = []string{"wi1", "wi2"}
		}

		// TODO: export this helper from the catalog package for testing.
		var status *pbresource.Status
		if withWIDs {
			status = &pbresource.Status{
				Conditions: []*pbresource.Condition{{
					Type:    catalog.StatusConditionBoundIdentities,
					State:   pbresource.Condition_STATE_TRUE,
					Message: strings.Join(ids, ","),
				}},
			}
		} else {
			status = &pbresource.Status{
				Conditions: []*pbresource.Condition{{
					Type:    catalog.StatusConditionBoundIdentities,
					State:   pbresource.Condition_STATE_TRUE,
					Message: "",
				}},
			}
		}
		r := rtest.Resource(pbcatalog.ServiceType, n).
			WithData(t, &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{
					Names: workloads,
				},
				Ports: []*pbcatalog.ServicePort{
					{TargetPort: "mesh", Protocol: pbcatalog.Protocol_PROTOCOL_MESH},
					{TargetPort: "grpc", Protocol: pbcatalog.Protocol_PROTOCOL_GRPC},
				},
			}).
			WithTenancy(tenancy).
			WithStatus(catalog.EndpointsStatusKey, status).
			Write(t, client)

		var statusUpdate = func() *pbresource.Resource { return r }
		if !withWIDs {
			statusUpdate = func() *pbresource.Resource {
				ctx := client.Context(t)

				status := &pbresource.Status{
					ObservedGeneration: r.Generation,
					Conditions: []*pbresource.Condition{{
						Type:    catalog.StatusConditionBoundIdentities,
						State:   pbresource.Condition_STATE_TRUE,
						Message: strings.Join(ids, ","),
					}},
				}
				resp, err := client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
					Id:     r.Id,
					Key:    catalog.EndpointsStatusKey,
					Status: status,
				})
				require.NoError(t, err)
				return resp.Resource
			}
		}

		sf := &serviceFixture{
			Resource:     r,
			StatusUpdate: statusUpdate,
		}

		rs = append(rs, sf)
	}
	return rs
}

func (suite *controllerSuite) createComputedRoutes(svc *pbresource.Resource, decResList ...any) *types.DecodedComputedRoutes {
	return createComputedRoutes(suite.T(), suite.client, svc, decResList...)
}

func createComputedRoutes(t *testing.T, client *rtest.Client, svc *pbresource.Resource, decResList ...any) *types.DecodedComputedRoutes {
	resList := make([]any, 0, len(decResList)+1)
	resList = append(resList,
		resourcetest.MustDecode[*pbcatalog.Service](t, svc),
	)
	resList = append(resList, decResList...)
	crID := resource.ReplaceType(pbmesh.ComputedRoutesType, svc.Id)
	cr := routestest.ReconcileComputedRoutes(t, client, crID, resList...)
	require.NotNil(t, cr)
	return cr
}

func (suite *controllerSuite) TestReconcile_CIDCreate_NoReferencingTrafficPermissionsExist() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		wi := rtest.Resource(pbauth.WorkloadIdentityType, "wi1").
			WithTenancy(tenancy).
			Write(suite.T(), suite.client)

		id := rtest.Resource(pbmesh.ComputedImplicitDestinationsType, wi.Id.Name).
			WithTenancy(tenancy).
			ID()

		suite.reconcileOnce(id)

		// Ensure that the CID was created
		cid := suite.client.RequireResourceExists(suite.T(), id)
		suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
			Destinations:    []*pbmesh.ImplicitDestination{},
			BoundReferences: nil,
		})
	})
}

func (suite *controllerSuite) TestReconcile_CIDCreate_ReferencingResourcesExist() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		suite.createTrafficPermissions([]string{"d-wi1-s-wi2"}, []string{"wi2"}, tenancy)
		resID := &pbresource.ID{
			Name:    "wi2",
			Type:    pbmesh.ComputedImplicitDestinationsType,
			Tenancy: tenancy,
		}
		// create the workload identity for the source
		wi := suite.createWorkloadIdentities([]string{"wi1", "wi2"}, tenancy)
		svc := suite.createServices([]string{"s1-2"}, tenancy, true)

		// Write a default ComputedRoutes for s1-2.
		cr := suite.createComputedRoutes(svc[0].Resource)

		ctpID := resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi[0].Id)

		suite.reconcileOnce(resID)

		// Ensure that the CID was created
		cid := suite.client.RequireResourceExists(suite.T(), resID)
		suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
			Destinations: []*pbmesh.ImplicitDestination{{
				DestinationRef:   refFromID(svc[0].Id),
				DestinationPorts: []string{"grpc"},
			}},
			BoundReferences: []*pbresource.Reference{
				refFromID(ctpID),
				refFromID(svc[0].Id),
				refFromID(cr.Id),
			},
		})
		rtest.RequireOwner(suite.T(), cid, wi[1].Id, true)
	})
}

const (
	omitCTP              = "computed-traffic-permissions"
	omitWorkloadIdentity = "workload-identity"
	omitService          = "service"
	omitWIOnService      = "wi-on-service"
	omitComputedRoutes   = "computed-routes"
)

func (suite *controllerSuite) TestReconcile_CIDCreate_IncrementalConstruction_ComputedTrafficPermissions() {
	suite.testReconcile_CIDCreate_IncrementalConstruction(omitCTP)
}

func (suite *controllerSuite) TestReconcile_CIDCreate_IncrementalConstruction_WorkloadIdentity() {
	suite.testReconcile_CIDCreate_IncrementalConstruction(omitWorkloadIdentity)
}

func (suite *controllerSuite) TestReconcile_CIDCreate_IncrementalConstruction_Service() {
	suite.testReconcile_CIDCreate_IncrementalConstruction(omitService)
}

func (suite *controllerSuite) TestReconcile_CIDCreate_IncrementalConstruction_WorkloadIdentitiesOnService() {
	suite.testReconcile_CIDCreate_IncrementalConstruction(omitWIOnService)
}

func (suite *controllerSuite) TestReconcile_CIDCreate_IncrementalConstruction_ComputedRoutes() {
	suite.testReconcile_CIDCreate_IncrementalConstruction(omitComputedRoutes)
}

func (suite *controllerSuite) testReconcile_CIDCreate_IncrementalConstruction(omit string) {
	// There are 5 major ingredients to assemble a CID that this test machinery cares about:
	//
	// - Workload Identities
	// - Computed Traffic Permissions
	// - Services
	// - Workload Identity data-bearing status cond on Services
	// - Computed Routes
	//
	// For each of these possible ingredients, we execute the test in 4 chunks:
	//
	// 1. Build everything *except* one ingredient.
	// 2. Reconcile and assert what the CID should look like without it.
	// 3. Build the omitted ingredient.
	// 4. Reconcile and assert that the CID looks the same in all cases.
	//
	// NOTEs:
	//
	// - CRs are owned by Services, so skipping the services will also skip the CRs.
	// - status conds live on the service, so late-adding them is different
	//   than creating them the first time.
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		resID := &pbresource.ID{
			Name:    "wi2",
			Type:    pbmesh.ComputedImplicitDestinationsType,
			Tenancy: tenancy,
		}

		var (
			wi  []*pbresource.Resource
			svc []*serviceFixture
			cr  *types.DecodedComputedRoutes
		)

		if omit != omitWorkloadIdentity {
			wi = suite.createWorkloadIdentities([]string{"wi1", "wi2"}, tenancy)
		}
		if omit != omitCTP {
			suite.createTrafficPermissions([]string{"d-wi1-s-wi2"}, []string{"wi2"}, tenancy)
		}
		if omit != omitService {
			svc = suite.createServices([]string{"s1-2"}, tenancy, (omit != omitWIOnService))
			if omit != omitComputedRoutes {
				// Write a default ComputedRoutes for s1-2.
				cr = suite.createComputedRoutes(svc[0].Resource)
			}
		}

		// Reconcile the first time with one omission.
		suite.reconcileOnce(resID)

		switch omit {
		case omitWorkloadIdentity:
			// Ensure that no CID was created
			suite.client.RequireResourceNotFound(suite.T(), resID)
		case omitCTP:
			// no bound resources at all
			cid := suite.client.RequireResourceExists(suite.T(), resID)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
			rtest.RequireOwner(suite.T(), cid, wi[1].Id, true)
		case omitService, omitWIOnService:
			// no linking workloads, no implicit destinations
			cid := suite.client.RequireResourceExists(suite.T(), resID)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
			rtest.RequireOwner(suite.T(), cid, wi[1].Id, true)
		case omitComputedRoutes:
			// no linking workloads, no implicit destinations
			cid := suite.client.RequireResourceExists(suite.T(), resID)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
			rtest.RequireOwner(suite.T(), cid, wi[1].Id, true)
		default:
			suite.T().Fatalf("omit=%s case not handled yet", omit)
		}

		// Create WI and ensure CID is created
		switch omit {
		case omitWorkloadIdentity:
			wi = suite.createWorkloadIdentities([]string{"wi1", "wi2"}, tenancy)
		case omitCTP:
			suite.createTrafficPermissions([]string{"d-wi1-s-wi2"}, []string{"wi2"}, tenancy)
		case omitService:
			svc = suite.createServices([]string{"s1-2"}, tenancy, true)
			// Write a default ComputedRoutes for s1-2.
			cr = suite.createComputedRoutes(svc[0].Resource)
		case omitWIOnService:
			// update the special bound WI status cond after the fact
			svc[0].StatusUpdate()
		case omitComputedRoutes:
			// Write a default ComputedRoutes for s1-2.
			cr = suite.createComputedRoutes(svc[0].Resource)
		default:
			suite.T().Fatalf("omit=%s case not handled yet", omit)
		}

		suite.reconcileOnce(resID)

		ctpID := resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi[0].Id)

		cid := suite.client.RequireResourceExists(suite.T(), resID)
		suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
			Destinations: []*pbmesh.ImplicitDestination{{
				DestinationRef:   refFromID(svc[0].Id),
				DestinationPorts: []string{"grpc"},
			}},
			BoundReferences: []*pbresource.Reference{
				refFromID(ctpID),
				refFromID(svc[0].Id),
				refFromID(cr.Id),
			},
		})
		rtest.RequireOwner(suite.T(), cid, wi[1].Id, true)
	})
}

func (suite *controllerSuite) TestReconcile_CIDUpdate_Multiple_Workloads_Services_TrafficPermissions() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		wi := suite.createWorkloadIdentities([]string{
			"wi1", "wi2", "wi3", "wi4", "wi5",
		}, tenancy)

		svc := suite.createServices([]string{
			"s1",
			"s2",
			"s3",
			"s4",
			"s1-2",
			"s11-2",
		}, tenancy, true)

		suite.createTrafficPermissions([]string{
			"d-wi1-s-wi3",
			"d-wi2-s-wi3",
			"d-wi1-s-wi2",
			"d-wi4-s-wi5",
		}, []string{"wi3", "wi5"}, tenancy)

		var (
			wi1 = wi[0]
			wi2 = wi[1]
			wi3 = wi[2]
			wi4 = wi[3]
			wi5 = wi[4]

			svc1    = svc[0]
			svc2    = svc[1]
			svc3    = svc[2]
			svc4    = svc[3]
			svc1_2  = svc[4]
			svc11_2 = svc[5]
		)

		var (
			cr1 = suite.createComputedRoutes(svc1.Resource)
			cr2 = suite.createComputedRoutes(svc2.Resource)
			cr3 = suite.createComputedRoutes(svc3.Resource,
				resourcetest.MustDecode[*pbmesh.GRPCRoute](suite.T(), rtest.Resource(pbmesh.GRPCRouteType, "grpc-route").
					WithTenancy(tenancy).
					WithData(suite.T(), &pbmesh.GRPCRoute{
						ParentRefs: []*pbmesh.ParentReference{{
							Ref: refFromID(svc3.Id),
						}},
						Rules: []*pbmesh.GRPCRouteRule{{
							BackendRefs: []*pbmesh.GRPCBackendRef{
								{
									BackendRef: &pbmesh.BackendReference{
										Ref: refFromID(svc3.Id),
									},
									Weight: 50,
								},
								{
									BackendRef: &pbmesh.BackendReference{
										Ref: refFromID(svc4.Id),
									},
									Weight: 50,
								},
							},
						}},
					}).
					Build()),
				resourcetest.MustDecode[*pbcatalog.Service](suite.T(), svc4.Resource),
			)
			cr4    = suite.createComputedRoutes(svc4.Resource)
			cr1_2  = suite.createComputedRoutes(svc1_2.Resource)
			cr11_2 = suite.createComputedRoutes(svc11_2.Resource)

			ctpID1 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi1.Id)
			ctpID2 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi2.Id)
			// ctpID3 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi3.Id)
			ctpID4 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi4.Id)
			// ctpID5 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi5.Id)

			cidWI1 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi1.Id)
			cidWI2 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi2.Id)
			cidWI3 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi3.Id)
			cidWI4 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi4.Id)
			cidWI5 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi5.Id)
		)

		suite.reconcileOnce(cidWI1)
		suite.reconcileOnce(cidWI2)
		suite.reconcileOnce(cidWI3)
		suite.reconcileOnce(cidWI4)
		suite.reconcileOnce(cidWI5)

		/*
			CTPs:
			wi3->wi1
			wi3->wi2
			wi2->wi1
			wi5->wi4

			WIs by SOURCE:
			wi1: []
			wi2: [wi1]
			wi3: [wi1, wi2]
			wi4: []
			wi5: [wi4]

			SVCs:
			s1:    [wi1]
			s2:    [wi2]
			s3:    [wi3]
			s4:    [wi4]
			s1-2:  [wi1, wi2]
			s11-2: [wi1, wi2]

			CRs:
			s1:    [s1]
			s2:    [s2]
			s3:    [s3, s4]
			s4:    [s4]
			s1-2:  [s1-2]
			s11-2: [s11-2]

			WIs by SOURCE + SVC+WI:
			wi1: []
			wi2: [s1, s1-2, s11-2]
			wi3: [s1, s2, s1-2, s11-2]
			wi4: []
			wi5: [s4]

			EXPECT CIDs:

			wi1: []
			wi2: [s1, s1-2, s11-2]
			wi3: [s1, s2, s1-2, s11-2]
			wi4: []
			wi5: [s3, s4]

		*/

		// Ensure that the CIDs were created
		suite.Run("cid for wi1", func() {
			cid := suite.client.RequireResourceExists(suite.T(), cidWI1)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
			rtest.RequireOwner(suite.T(), cid, wi1.Id, true)
		})

		suite.Run("cid for wi2", func() {
			cid := suite.client.RequireResourceExists(suite.T(), cidWI2)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					// { s1, refs: wi1 }, { s1-2, refs: wi1 }, { s11-2, refs: wi1 }
					{DestinationRef: refFromID(svc1.Id), DestinationPorts: []string{"grpc"}},
					{DestinationRef: refFromID(svc1_2.Id), DestinationPorts: []string{"grpc"}},
					{DestinationRef: refFromID(svc11_2.Id), DestinationPorts: []string{"grpc"}},
				},
				BoundReferences: []*pbresource.Reference{
					refFromID(ctpID1),

					refFromID(svc1.Id),
					refFromID(svc1_2.Id),
					refFromID(svc11_2.Id),

					refFromID(cr1.Id),
					refFromID(cr1_2.Id),
					refFromID(cr11_2.Id),
				},
			})
			rtest.RequireOwner(suite.T(), cid, wi2.Id, true)
		})

		suite.Run("cid for wi3", func() {
			cid := suite.client.RequireResourceExists(suite.T(), cidWI3)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					// { s1, refs: wi1 }, { s2, refs: wi2 }, { s1-2, refs: wi1, wi2 }, { s11-2, refs: wi1, wi2 }
					{DestinationRef: refFromID(svc1.Id), DestinationPorts: []string{"grpc"}},
					{DestinationRef: refFromID(svc1_2.Id), DestinationPorts: []string{"grpc"}},
					{DestinationRef: refFromID(svc11_2.Id), DestinationPorts: []string{"grpc"}},
					{DestinationRef: refFromID(svc2.Id), DestinationPorts: []string{"grpc"}},
				},
				BoundReferences: []*pbresource.Reference{
					refFromID(ctpID1),
					refFromID(ctpID2),

					refFromID(svc1.Id),
					refFromID(svc1_2.Id),
					refFromID(svc11_2.Id),
					refFromID(svc2.Id),

					refFromID(cr1.Id),
					refFromID(cr1_2.Id),
					refFromID(cr11_2.Id),
					refFromID(cr2.Id),
				},
			})
			rtest.RequireOwner(suite.T(), cid, wi3.Id, true)
		})

		suite.Run("cid for wi4", func() {
			cid := suite.client.RequireResourceExists(suite.T(), cidWI4)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
			rtest.RequireOwner(suite.T(), cid, wi4.Id, true)
		})

		suite.Run("cid for wi5", func() {
			cid := suite.client.RequireResourceExists(suite.T(), cidWI5)
			suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
				Destinations: []*pbmesh.ImplicitDestination{
					// { s4, refs: wi4 }, { s3, refs: wi3 }
					{DestinationRef: refFromID(svc3.Id), DestinationPorts: []string{"grpc"}},
					{DestinationRef: refFromID(svc4.Id), DestinationPorts: []string{"grpc"}},
				},
				BoundReferences: []*pbresource.Reference{
					refFromID(ctpID4),

					refFromID(svc3.Id),
					refFromID(svc4.Id),

					refFromID(cr3.Id),
					refFromID(cr4.Id),
				},
			})
			rtest.RequireOwner(suite.T(), cid, wi5.Id, true)
		})
	})
}

func (suite *controllerSuite) reconcileOnce(id *pbresource.ID) {
	err := suite.ctl.Reconcile(suite.ctx, controller.Request{ID: id})
	require.NoError(suite.T(), err)
	suite.T().Cleanup(func() {
		suite.client.CleanupDelete(suite.T(), id)
	})
}

func (suite *controllerSuite) TestReconcile_CIDUpdate_TrafficPermissions_WildcardName() {
	if !suite.isEnterprise {
		suite.T().Skip("test only applies in enterprise as written")
	}
	fixedTenancy := rtest.Tenancy("default.fixed")

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		wi := suite.createWorkloadIdentities([]string{
			"wi1", "wi2", "wi3",
		}, tenancy)

		wiFixed := suite.createWorkloadIdentities([]string{
			"wi4", "wi5",
		}, fixedTenancy)

		svc := suite.createServices([]string{"s1", "s2", "s3"}, tenancy, true)

		var (
			wi1 = wi[0]
			wi2 = wi[1]
			wi3 = wi[2]

			wiFixed4 = wiFixed[0]
			wiFixed5 = wiFixed[1]

			svc1 = svc[0]
			svc2 = svc[1]
			svc3 = svc[2]
		)

		var (
			_   = suite.createComputedRoutes(svc1.Resource)
			_   = suite.createComputedRoutes(svc2.Resource)
			cr3 = suite.createComputedRoutes(svc3.Resource)
		)

		// Create some stub CTPs.
		suite.createTrafficPermissions([]string{}, []string{"wi1", "wi2"}, tenancy)
		suite.createTrafficPermissions([]string{}, []string{"wi4", "wi5"}, fixedTenancy)

		// Create a wildcard "all names in a namespace" TP.
		ctpID3 := ReconcileComputedTrafficPermissions(
			suite.T(),
			suite.client,
			resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi3.Id),
			&pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: "",
						Namespace:    fixedTenancy.Namespace,
						Partition:    fixedTenancy.Partition,
					}},
				}},
			},
		).Id

		// These DO NOT match the wildcard.
		for _, wiID := range []*pbresource.ID{wi1.Id, wi2.Id, wi3.Id} {
			cidWI := resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiID)

			suite.reconcileOnce(cidWI)

			suite.Run("empty cid for "+resource.IDToString(wiID), func() {
				cid := suite.client.RequireResourceExists(suite.T(), cidWI)
				suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
				rtest.RequireOwner(suite.T(), cid, wiID, true)
			})
		}

		// These DO match the wildcard.
		for _, wiID := range []*pbresource.ID{wiFixed4.Id, wiFixed5.Id} {
			cidWI := resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiID)

			suite.reconcileOnce(cidWI)

			suite.Run("cid for "+resource.IDToString(wiID), func() {
				cid := suite.client.RequireResourceExists(suite.T(), cidWI)
				suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{{
						DestinationRef:   refFromID(svc3.Id),
						DestinationPorts: []string{"grpc"},
					}},
					BoundReferences: []*pbresource.Reference{
						refFromID(ctpID3),
						refFromID(svc3.Id),
						refFromID(cr3.Id),
					},
				})
				rtest.RequireOwner(suite.T(), cid, wiID, true)
			})
		}
	})
}

func (suite *controllerSuite) TestReconcile_CIDUpdate_TrafficPermissions_WildcardNamespace() {
	if !suite.isEnterprise {
		suite.T().Skip("test only applies in enterprise as written")
	}

	wildTenancy1 := rtest.Tenancy("wild.aaa")
	wildTenancy2 := rtest.Tenancy("wild.bbb")

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		wi := suite.createWorkloadIdentities([]string{
			"wi1", "wi2", "wi3",
		}, tenancy)

		wiWild4 := suite.createWorkloadIdentities([]string{
			"wi4",
		}, wildTenancy1)[0]

		wiWild5 := suite.createWorkloadIdentities([]string{
			"wi5",
		}, wildTenancy2)[0]

		svc := suite.createServices([]string{"s1", "s2", "s3"}, tenancy, true)

		var (
			wi1 = wi[0]
			wi2 = wi[1]
			wi3 = wi[2]

			svc1 = svc[0]
			svc2 = svc[1]
			svc3 = svc[2]
		)

		var (
			_   = suite.createComputedRoutes(svc1.Resource)
			_   = suite.createComputedRoutes(svc2.Resource)
			cr3 = suite.createComputedRoutes(svc3.Resource)
		)

		// Create some stub CTPs.
		suite.createTrafficPermissions([]string{}, []string{"wi1", "wi2"}, tenancy)
		suite.createTrafficPermissions([]string{}, []string{"wi4"}, wildTenancy1)
		suite.createTrafficPermissions([]string{}, []string{"wi5"}, wildTenancy2)

		// Create a wildcard "all namespaces in a partition" TP.
		ctpID3 := ReconcileComputedTrafficPermissions(
			suite.T(),
			suite.client,
			resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi3.Id),
			&pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: "",
						Namespace:    "",
						Partition:    "wild",
					}},
				}},
			},
		).Id

		// These DO NOT match the wildcard.
		for _, wiID := range []*pbresource.ID{wi1.Id, wi2.Id, wi3.Id} {
			cidWI := resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiID)

			suite.reconcileOnce(cidWI)

			suite.Run("empty cid for "+resource.IDToString(wiID), func() {
				cid := suite.client.RequireResourceExists(suite.T(), cidWI)
				suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{})
				rtest.RequireOwner(suite.T(), cid, wiID, true)
			})
		}

		// These DO match the wildcard.
		for _, wiID := range []*pbresource.ID{wiWild4.Id, wiWild5.Id} {
			cidWI := resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiID)

			suite.reconcileOnce(cidWI)

			suite.Run("cid for "+resource.IDToString(wiID), func() {
				cid := suite.client.RequireResourceExists(suite.T(), cidWI)
				suite.requireCID(cid, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{{
						DestinationRef:   refFromID(svc3.Id),
						DestinationPorts: []string{"grpc"},
					}},
					BoundReferences: []*pbresource.Reference{
						refFromID(ctpID3),
						refFromID(svc3.Id),
						refFromID(cr3.Id),
					},
				})
				rtest.RequireOwner(suite.T(), cid, wiID, true)
			})
		}
	})
}

// TODO: test a bound references dep mapper loop

func (suite *controllerSuite) TestMapping() {
	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		////////////////////////////////////
		// Creating a WI triggers aligned reconcile.
		wi := suite.createWorkloadIdentities([]string{
			"wi1", "wi2", "wi3", "wi4",
		}, tenancy)
		var (
			wi1 = wi[0]
			wi2 = wi[1]
			wi3 = wi[2]
			wi4 = wi[3]

			cidWI1 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi1.Id)
			cidWI2 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi2.Id)
			cidWI3 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi3.Id)
			cidWI4 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi4.Id)

			ctpID1 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi1.Id)
			ctpID2 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi2.Id)
			ctpID3 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi3.Id)
			ctpID4 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi4.Id)
		)
		suite.assertMapperDefaultDeny(wi1, cidWI1)
		suite.assertMapperDefaultDeny(wi2, cidWI2)
		suite.assertMapperDefaultDeny(wi3, cidWI3)
		suite.assertMapperDefaultDeny(wi4, cidWI4)

		suite.assertMapperDefaultAllow(wi1, cidWI1)
		suite.assertMapperDefaultAllow(wi2, cidWI2)
		suite.assertMapperDefaultAllow(wi3, cidWI3)
		suite.assertMapperDefaultAllow(wi4, cidWI4)

		////////////////////////////////////
		// Creating a CTP that references a wi as a source triggers.
		suite.createTrafficPermissions([]string{
			"d-wi2-s-wi1",
			"d-wi1-s-wi2",
		}, []string{"wi4"}, tenancy)

		// Create a wildcard "all names in a namespace" TP.
		ReconcileComputedTrafficPermissions(
			suite.T(),
			suite.client,
			ctpID3,
			&pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: "",
						// Namespace:    tenancy.Namespace,
						// Partition:    tenancy.Partition,
					}},
				}},
			},
		)

		var (
			ctp1 = suite.client.RequireResourceExists(suite.T(), ctpID1)
			ctp2 = suite.client.RequireResourceExists(suite.T(), ctpID2)
			ctp3 = suite.client.RequireResourceExists(suite.T(), ctpID3)
			ctp4 = suite.client.RequireResourceExists(suite.T(), ctpID4)
		)
		suite.assertMapperDefaultDeny(ctp1, cidWI2)
		suite.assertMapperDefaultDeny(ctp2, cidWI1)
		suite.assertMapperDefaultDeny(ctp3, cidWI1, cidWI2, cidWI3, cidWI4)
		suite.assertMapperDefaultDeny(ctp4)

		suite.assertMapperDefaultAllow(ctp1, cidWI2)
		suite.assertMapperDefaultAllow(ctp2, cidWI1)
		suite.assertMapperDefaultAllow(ctp3, cidWI1, cidWI2, cidWI3, cidWI4)
		// wi4 has a default CTP in default allow so it allows all traffic to it.
		suite.assertMapperDefaultAllow(ctp4, cidWI1, cidWI2, cidWI3, cidWI4)

		////////////////////////////////////
		// Creating a Service alone does nothing.
		svc := suite.createServices([]string{
			"s1", "s2", "s3", "s4",
		}, tenancy, false)
		var (
			svc1 = svc[0]
			svc2 = svc[1]
			svc3 = svc[2]
			svc4 = svc[3]
		)
		suite.assertMapperDefaultDeny(svc1.Resource)
		suite.assertMapperDefaultDeny(svc2.Resource)
		suite.assertMapperDefaultDeny(svc3.Resource)
		suite.assertMapperDefaultDeny(svc4.Resource)

		suite.assertMapperDefaultAllow(svc1.Resource)
		suite.assertMapperDefaultAllow(svc2.Resource)
		suite.assertMapperDefaultAllow(svc3.Resource)
		suite.assertMapperDefaultAllow(svc4.Resource)

		////////////////////////////////////
		// Have to update the special status condition first.
		svc1.Resource = svc1.StatusUpdate()
		svc2.Resource = svc2.StatusUpdate()
		svc3.Resource = svc3.StatusUpdate()
		svc4.Resource = svc4.StatusUpdate()
		suite.assertMapperDefaultDeny(svc1.Resource, cidWI2)
		suite.assertMapperDefaultDeny(svc2.Resource, cidWI1)
		suite.assertMapperDefaultDeny(svc3.Resource, cidWI1, cidWI2, cidWI3, cidWI4)
		suite.assertMapperDefaultDeny(svc4.Resource)

		suite.assertMapperDefaultAllow(svc1.Resource, cidWI2)
		suite.assertMapperDefaultAllow(svc2.Resource, cidWI1)
		suite.assertMapperDefaultAllow(svc3.Resource, cidWI1, cidWI2, cidWI3, cidWI4)
		// s4 maps to wi4 which has a default CTP in default allow so it allows all traffic to it.
		suite.assertMapperDefaultAllow(svc4.Resource, cidWI1, cidWI2, cidWI3, cidWI4)

		////////////////////////////////////
		// Add a computed routes that provides another alias for the workloads.
		cr1 := suite.createComputedRoutes(svc1.Resource)
		cr2 := suite.createComputedRoutes(svc2.Resource)
		cr3 := suite.createComputedRoutes(svc3.Resource,
			resourcetest.MustDecode[*pbmesh.GRPCRoute](suite.T(), rtest.Resource(pbmesh.GRPCRouteType, "grpc-route").
				WithTenancy(tenancy).
				WithData(suite.T(), &pbmesh.GRPCRoute{
					ParentRefs: []*pbmesh.ParentReference{{
						Ref: refFromID(svc3.Id),
					}},
					Rules: []*pbmesh.GRPCRouteRule{{
						BackendRefs: []*pbmesh.GRPCBackendRef{
							{
								BackendRef: &pbmesh.BackendReference{
									Ref: refFromID(svc1.Id),
								},
								Weight: 50,
							},
							{
								BackendRef: &pbmesh.BackendReference{
									Ref: refFromID(svc2.Id),
								},
								Weight: 50,
							},
						},
					}},
				}).
				Build()),
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), svc1.Resource),
			resourcetest.MustDecode[*pbcatalog.Service](suite.T(), svc2.Resource),
		)
		cr4 := suite.createComputedRoutes(svc4.Resource)
		suite.assertMapperDefaultDeny(cr1.Resource, cidWI2)
		suite.assertMapperDefaultDeny(cr2.Resource, cidWI1)
		suite.assertMapperDefaultDeny(cr3.Resource, cidWI1, cidWI2)
		suite.assertMapperDefaultDeny(cr4.Resource)

		suite.assertMapperDefaultAllow(cr1.Resource, cidWI2)
		suite.assertMapperDefaultAllow(cr2.Resource, cidWI1)
		suite.assertMapperDefaultAllow(cr3.Resource, cidWI1, cidWI2)
		// cr4 aligns to s4 which maps to wi4 which has a default CTP in default allow so it allows all traffic to it.
		suite.assertMapperDefaultAllow(cr4.Resource, cidWI1, cidWI2, cidWI3, cidWI4)
	})
}

func (suite *controllerSuite) TestMapping_WildcardNamesAndNamespaces() {
	if !suite.isEnterprise {
		suite.T().Skip("test only applies in enterprise as written")
	}

	fixedTenancy := rtest.Tenancy("default.fixed") // for wildcard name
	wildTenancy1 := rtest.Tenancy("wild.aaa")      // for wildcard ns
	wildTenancy2 := rtest.Tenancy("wild.bbb")      // for wildcard ns

	suite.runTestCaseWithTenancies(func(tenancy *pbresource.Tenancy) {
		// Creating a WI triggers aligned reconcile.
		wi := suite.createWorkloadIdentities([]string{
			"wi1", "wi2", "wi3",
		}, tenancy)

		wiWild4 := suite.createWorkloadIdentities([]string{
			"wi4",
		}, wildTenancy1)[0]

		wiWild5 := suite.createWorkloadIdentities([]string{
			"wi5",
		}, wildTenancy2)[0]

		wiFixed := suite.createWorkloadIdentities([]string{
			"wi4", "wi5",
		}, fixedTenancy)

		svc := suite.createServices([]string{"s1", "s2", "s3"}, tenancy, true)

		var (
			wi1 = wi[0]
			wi2 = wi[1]
			wi3 = wi[2]

			wiFixed4 = wiFixed[0]
			wiFixed5 = wiFixed[1]

			svc1 = svc[0]
			svc2 = svc[1]
			svc3 = svc[2]

			cidWI1 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi1.Id)
			cidWI2 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi2.Id)
			cidWI3 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi3.Id)

			cidWIWild4 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiWild4.Id)
			cidWIWild5 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiWild5.Id)

			cidWIFixed4 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiFixed4.Id)
			cidWIFixed5 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wiFixed5.Id)
		)

		var (
			cr1 = suite.createComputedRoutes(svc1.Resource)
			cr2 = suite.createComputedRoutes(svc2.Resource)
			cr3 = suite.createComputedRoutes(svc3.Resource)
		)

		// Create some stub CTPs.
		suite.createTrafficPermissions([]string{}, []string{"wi1"}, tenancy)
		suite.createTrafficPermissions([]string{}, []string{"wi4"}, wildTenancy1)
		suite.createTrafficPermissions([]string{}, []string{"wi5"}, wildTenancy2)

		var (
			ctpID1 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi1.Id)
		)

		// Create a wildcard "all names in a namespace" TP.
		ctpID2 := ReconcileComputedTrafficPermissions(
			suite.T(),
			suite.client,
			resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi2.Id),
			&pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: "",
						Namespace:    fixedTenancy.Namespace,
						Partition:    fixedTenancy.Partition,
					}},
				}},
			},
		).Id

		// Create a wildcard "all namespaces in a partition" TP.
		ctpID3 := ReconcileComputedTrafficPermissions(
			suite.T(),
			suite.client,
			resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi3.Id),
			&pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: "",
						Namespace:    "",
						Partition:    "wild",
					}},
				}},
			},
		).Id

		var (
			ctp1 = suite.client.RequireResourceExists(suite.T(), ctpID1)
			ctp2 = suite.client.RequireResourceExists(suite.T(), ctpID2)
			ctp3 = suite.client.RequireResourceExists(suite.T(), ctpID3)
		)

		////////////////////////////////////
		// Workload Identities
		// (nothing interesting here)

		suite.assertMapperDefaultDeny(wi1, cidWI1)
		suite.assertMapperDefaultDeny(wi2, cidWI2)
		suite.assertMapperDefaultDeny(wi3, cidWI3)

		////////////////////////////////////
		// CTPs
		// (ctp2 aligns with wi2 which has the default.fixed.* allow)
		// (ctp3 aligns with wi3 which has the wild.*.* allow)

		suite.assertMapperDefaultDeny(ctp1)
		suite.assertMapperDefaultDeny(ctp2, cidWIFixed4, cidWIFixed5)
		suite.assertMapperDefaultDeny(ctp3, cidWIWild4, cidWIWild5)

		////////////////////////////////////
		// Services
		// (s2 encompasses wi2 which has the default.fixed.* allow)
		// (s3 encompasses wi3 which has the wild.*.* allow)

		suite.assertMapperDefaultDeny(svc1.Resource)
		suite.assertMapperDefaultDeny(svc2.Resource, cidWIFixed4, cidWIFixed5)
		suite.assertMapperDefaultDeny(svc3.Resource, cidWIWild4, cidWIWild5)

		////////////////////////////////////
		// Computed Routes
		// (cr2 aligns with s2 which encompasses wi2 which has the default.fixed.* allow)
		// (cr3 aligns with s3 which encompasses wi3 which has the wild.*.* allow)

		suite.assertMapperDefaultDeny(cr1.Resource)
		suite.assertMapperDefaultDeny(cr2.Resource, cidWIFixed4, cidWIFixed5)
		suite.assertMapperDefaultDeny(cr3.Resource, cidWIWild4, cidWIWild5)
	})
}

func TestController_DefaultDeny(t *testing.T) {
	// This test's purpose is to exercise the controller in a halfway realistic
	// way. Generally we are trying to go through the whole lifecycle of the
	// controller.
	//
	// This isn't a full integration test as that would require also executing
	// various other controllers.

	clientRaw := controllertest.NewControllerTestBuilder().
		WithTenancies(resourcetest.TestTenancies()...).
		WithResourceRegisterFns(types.Register, catalog.RegisterTypes, auth.RegisterTypes).
		WithControllerRegisterFns(func(mgr *controller.Manager) {
			mgr.Register(Controller(false))
		}).
		Run(t)

	client := rtest.NewClient(clientRaw)

	for _, tenancy := range resourcetest.TestTenancies() {
		t.Run(tenancySubTestName(tenancy), func(t *testing.T) {
			tenancy := tenancy

			// Add some workload identities and services.
			wi := createWorkloadIdentities(t, client, []string{
				"wi1", "wi2", "wi3",
			}, tenancy)
			var (
				wi1 = wi[0]
				wi2 = wi[1]
				wi3 = wi[2]

				// ctpID1 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi1.Id)
				ctpID2 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi2.Id)
				ctpID3 = resource.ReplaceType(pbauth.ComputedTrafficPermissionsType, wi3.Id)

				cidID1 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi1.Id)
				cidID2 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi2.Id)
				cidID3 = resource.ReplaceType(pbmesh.ComputedImplicitDestinationsType, wi3.Id)
			)
			svc := createServices(t, client, []string{"s1", "s2", "s3"}, tenancy, true)
			var (
				_    = svc[0]
				svc2 = svc[1]
				svc3 = svc[2]

				crID2 = resource.ReplaceType(pbmesh.ComputedRoutesType, svc2.Id)
				crID3 = resource.ReplaceType(pbmesh.ComputedRoutesType, svc3.Id)
			)

			// Wait for the empty stub resources to be created.
			cidVersion1 := requireNewCIDVersion(t, client, cidID1, "", &pbmesh.ComputedImplicitDestinations{})
			_ = requireNewCIDVersion(t, client, cidID2, "", &pbmesh.ComputedImplicitDestinations{})
			_ = requireNewCIDVersion(t, client, cidID3, "", &pbmesh.ComputedImplicitDestinations{})

			// Add some other required resources.

			ReconcileComputedTrafficPermissions(t, client, ctpID2, &pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: wi1.Id.Name,
					}},
				}},
			})
			createComputedRoutes(t, client, svc2.Resource)

			testutil.RunStep(t, "wi1 can reach wi2", func(t *testing.T) {
				cidVersion1 = requireNewCIDVersion(t, client, cidID1, cidVersion1, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{{
						DestinationRef:   resource.Reference(svc2.Id, ""),
						DestinationPorts: []string{"grpc"},
					}},
					BoundReferences: []*pbresource.Reference{
						resource.Reference(ctpID2, ""),
						resource.Reference(svc2.Id, ""),
						resource.Reference(crID2, ""),
					},
				})
			})

			ReconcileComputedTrafficPermissions(t, client, ctpID3, &pbauth.TrafficPermissions{
				Action: pbauth.Action_ACTION_ALLOW,
				Permissions: []*pbauth.Permission{{
					Sources: []*pbauth.Source{{
						IdentityName: wi1.Id.Name,
					}},
				}},
			})
			createComputedRoutes(t, client, svc3.Resource)

			testutil.RunStep(t, "wi1 can reach wi2 and wi3", func(t *testing.T) {
				cidVersion1 = requireNewCIDVersion(t, client, cidID1, cidVersion1, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{
						{
							DestinationRef:   resource.Reference(svc2.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
						{
							DestinationRef:   resource.Reference(svc3.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
					},
					BoundReferences: []*pbresource.Reference{
						resource.Reference(ctpID2, ""),
						resource.Reference(ctpID3, ""),
						resource.Reference(svc2.Id, ""),
						resource.Reference(svc3.Id, ""),
						resource.Reference(crID2, ""),
						resource.Reference(crID3, ""),
					},
				})
			})

			// Remove a route.
			client.MustDelete(t, crID2)

			testutil.RunStep(t, "removing a ComputedRoutes should remove that service from any CID", func(t *testing.T) {
				cidVersion1 = requireNewCIDVersion(t, client, cidID1, cidVersion1, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{
						{
							DestinationRef:   resource.Reference(svc3.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
					},
					BoundReferences: []*pbresource.Reference{
						resource.Reference(ctpID3, ""),
						resource.Reference(svc3.Id, ""),
						resource.Reference(crID3, ""),
					},
				})
			})

			// Put it back.
			createComputedRoutes(t, client, svc2.Resource)

			testutil.RunStep(t, "put the ComputedRoutes back", func(t *testing.T) {
				cidVersion1 = requireNewCIDVersion(t, client, cidID1, cidVersion1, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{
						{
							DestinationRef:   resource.Reference(svc2.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
						{
							DestinationRef:   resource.Reference(svc3.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
					},
					BoundReferences: []*pbresource.Reference{
						resource.Reference(ctpID2, ""),
						resource.Reference(ctpID3, ""),
						resource.Reference(svc2.Id, ""),
						resource.Reference(svc3.Id, ""),
						resource.Reference(crID2, ""),
						resource.Reference(crID3, ""),
					},
				})
			})

			// Remove traffic access to wi3.
			client.MustDelete(t, ctpID3)

			testutil.RunStep(t, "removing a CTP should remove those services only exposing that WI", func(t *testing.T) {
				cidVersion1 = requireNewCIDVersion(t, client, cidID1, cidVersion1, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{
						{
							DestinationRef:   resource.Reference(svc2.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
					},
					BoundReferences: []*pbresource.Reference{
						resource.Reference(ctpID2, ""),
						resource.Reference(svc2.Id, ""),
						resource.Reference(crID2, ""),
					},
				})
			})

			// Edit the route on top of svc3 to also split to svc2, which will
			// cause it to re-manifest as an implicit destination due to half of
			// the traffic possibly going to wi3.
			grpcRoute := rtest.Resource(pbmesh.GRPCRouteType, "grpc-route-3").
				WithTenancy(tenancy).
				WithData(t, &pbmesh.GRPCRoute{
					ParentRefs: []*pbmesh.ParentReference{{
						Ref:  resource.Reference(svc3.Id, ""),
						Port: "grpc",
					}},
					Rules: []*pbmesh.GRPCRouteRule{{
						BackendRefs: []*pbmesh.GRPCBackendRef{
							{
								BackendRef: &pbmesh.BackendReference{
									Ref: resource.Reference(svc2.Id, ""),
								},
								Weight: 50,
							},
							{
								BackendRef: &pbmesh.BackendReference{
									Ref: resource.Reference(svc3.Id, ""),
								},
								Weight: 50,
							},
						},
					}},
				}).
				Write(t, client)
			createComputedRoutes(t, client, svc3.Resource,
				rtest.MustDecode[*pbmesh.GRPCRoute](t, grpcRoute),
				rtest.MustDecode[*pbcatalog.Service](t, svc2.Resource),
			)

			testutil.RunStep(t, "a workload reachable by one branch of a computed routes still is implicit", func(t *testing.T) {
				cidVersion1 = requireNewCIDVersion(t, client, cidID1, cidVersion1, &pbmesh.ComputedImplicitDestinations{
					Destinations: []*pbmesh.ImplicitDestination{
						{
							DestinationRef:   resource.Reference(svc2.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
						{
							DestinationRef:   resource.Reference(svc3.Id, ""),
							DestinationPorts: []string{"grpc"},
						},
					},
					BoundReferences: []*pbresource.Reference{
						resource.Reference(ctpID2, ""),
						// no contribution to ctpID3, b/c it is deleted
						resource.Reference(svc2.Id, ""),
						resource.Reference(svc3.Id, ""),
						resource.Reference(crID2, ""),
						resource.Reference(crID3, ""),
					},
				})
			})
		})
	}
}

func (suite *controllerSuite) runStep(name string, fn func()) {
	suite.T().Helper()
	require.True(suite.T(), suite.Run(name, fn))
}

func requireNewCIDVersion(
	t *testing.T,
	client *rtest.Client,
	id *pbresource.ID,
	version string,
	expected *pbmesh.ComputedImplicitDestinations,
) string {
	t.Helper()

	var nextVersion string
	retry.Run(t, func(r *retry.R) {
		res := client.WaitForNewVersion(r, id, version)

		cid := rtest.MustDecode[*pbmesh.ComputedImplicitDestinations](r, res)

		prototest.AssertDeepEqual(r, expected, cid.Data)

		nextVersion = res.Version
	})
	return nextVersion
}

func (suite *controllerSuite) runTestCaseWithTenancies(testFunc func(*pbresource.Tenancy)) {
	for _, tenancy := range suite.tenancies {
		suite.Run(suite.appendTenancyInfo(tenancy), func() {
			testFunc(tenancy)
		})
	}
}

func (suite *controllerSuite) appendTenancyInfo(tenancy *pbresource.Tenancy) string {
	return tenancySubTestName(tenancy)
}

func tenancySubTestName(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func TestController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}

func refFromID(id *pbresource.ID) *pbresource.Reference {
	return resource.Reference(id, "")
}

func (suite *controllerSuite) assertMapperDefaultDeny(res *pbresource.Resource, expect ...*pbresource.ID) {
	suite.T().Helper()
	suite.assertMapper(suite.ctl, res, expect...)
}

func (suite *controllerSuite) assertMapperDefaultAllow(res *pbresource.Resource, expect ...*pbresource.ID) {
	suite.T().Helper()
	suite.assertMapper(suite.ctlDefaultAllow, res, expect...)
}

func (suite *controllerSuite) assertMapper(
	ctl *controller.TestController,
	res *pbresource.Resource,
	expect ...*pbresource.ID,
) {
	suite.T().Helper()
	reqs, err := ctl.DryRunMapper(suite.ctx, res)
	require.NoError(suite.T(), err)

	var got []*pbresource.ID
	for _, req := range reqs {
		got = append(got, req.ID)
	}

	prototest.AssertElementsMatch(suite.T(), expect, got)
}
