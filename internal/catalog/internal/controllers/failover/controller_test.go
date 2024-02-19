// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failover

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/failover/expander"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/multicluster"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestController(t *testing.T) {
	// This test's purpose is to exercise the controller in a halfway realistic
	// way, verifying the event triggers work in the live code.

	clientRaw := controllertest.NewControllerTestBuilder().
		WithTenancies(resourcetest.TestTenancies()...).
		WithResourceRegisterFns(types.Register, multicluster.RegisterTypes).
		WithControllerRegisterFns(func(mgr *controller.Manager) {
			mgr.Register(FailoverPolicyController(expander.GetSamenessGroupExpander()))
		}).
		Run(t)

	client := rtest.NewClient(clientRaw)

	for _, tenancy := range resourcetest.TestTenancies() {
		t.Run(tenancySubTestName(tenancy), func(t *testing.T) {
			tenancy := tenancy

			// Create an advance pointer to some services.
			apiServiceRef := resource.Reference(rtest.Resource(pbcatalog.ServiceType, "api").WithTenancy(tenancy).ID(), "")
			otherServiceRef := resource.Reference(rtest.Resource(pbcatalog.ServiceType, "other").WithTenancy(tenancy).ID(), "")

			// create a failover without any services
			failoverData := &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{{
						Ref: apiServiceRef,
					}},
				},
			}
			failover := rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, failover.Id) })

			var expectedComputedFP *pbcatalog.ComputedFailoverPolicy

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionMissingService)
			client.RequireResourceNotFound(t, resource.ReplaceType(pbcatalog.ComputedFailoverPolicyType, failover.Id))
			t.Logf("reconciled to missing service status")

			// Provide the service.
			apiServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{{
					VirtualPort: 8080,
					TargetPort:  "http",
					Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
				}},
			}
			svc := rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
				},
				BoundReferences: []*pbresource.Reference{apiServiceRef},
			}

			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionOK)

			t.Log("delete service")

			client.MustDelete(t, svc.Id)

			client.WaitForReconciliation(t, resource.ReplaceType(pbcatalog.ComputedFailoverPolicyType, failover.Id), ControllerID)
			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionMissingService)
			client.RequireResourceNotFound(t, resource.ReplaceType(pbcatalog.ComputedFailoverPolicyType, failover.Id))

			// re add the service
			rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Logf("reconciled to accepted")

			// Update the failover to reference a port twice (once by virtual, once by target port)
			failoverData = &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
					"8080": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
				},
			}
			failover = rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, failover.Id) })

			// Assert that the FailoverPolicy has the conflict condition.
			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionConflictDestinationPort(apiServiceRef, &pbcatalog.ServicePort{
				VirtualPort: 8080,
				TargetPort:  "http",
			}))

			// Assert that the ComputedFailoverPolicy has the conflict condition.
			// The port normalization that occurs in the call to SimplifyFailoverPolicy results in the port being
			// removed from the final FailoverPolicy and ComputedFailoverPolicy.
			expFailoverData := &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
				},
			}
			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs:     expFailoverData.PortConfigs,
				BoundReferences: []*pbresource.Reference{apiServiceRef},
			}
			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionConflictDestinationPort(apiServiceRef, &pbcatalog.ServicePort{
				VirtualPort: 8080,
				TargetPort:  "http",
			}))
			t.Logf("reconciled to using duplicate destination port")

			// Update the failover to fix the duplicate, but reference an unknown port
			failoverData = &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
					"admin": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "admin",
						}},
					},
				},
			}
			failover = rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, failover.Id) })

			// Assert that the FailoverPolicy has the unknown condition.
			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionUnknownPort(apiServiceRef, "admin"))

			// Assert that the ComputedFailoverPolicy has the unknown condition.
			// The port normalization that occurs in the call to SimplifyFailoverPolicy results in the port being
			// removed from the final FailoverPolicy and ComputedFailoverPolicy.
			expFailoverData = &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
				},
			}
			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs:     expFailoverData.PortConfigs,
				BoundReferences: []*pbresource.Reference{apiServiceRef},
			}
			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionUnknownPort(apiServiceRef, "admin"))
			t.Logf("reconciled to unknown admin port")

			// update the service to fix the stray reference, but point to a mesh port
			apiServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort:  "http",
						VirtualPort: 8080,
						Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort:  "admin",
						VirtualPort: 10000,
						Protocol:    pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
			}
			// update the expected ComputedFailoverPolicy to add back in the admin port as well
			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs:     failoverData.PortConfigs,
				BoundReferences: []*pbresource.Reference{apiServiceRef},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionUsingMeshDestinationPort(apiServiceRef, "admin"))
			t.Logf("reconciled to using mesh destination port")

			// update the service to fix the stray reference to not be a mesh port
			apiServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						VirtualPort: 8080,
						TargetPort:  "http",
						Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						VirtualPort: 10000,
						TargetPort:  "admin",
						Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionOK)
			t.Logf("reconciled to accepted")

			// change failover leg to point to missing service
			failoverData = &pbcatalog.FailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
					"admin": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  otherServiceRef,
							Port: "admin",
						}},
					},
				},
			}
			failover = rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, failover.Id) })

			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"http": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  apiServiceRef,
							Port: "http",
						}},
					},
				},
				BoundReferences: []*pbresource.Reference{apiServiceRef, otherServiceRef},
			}

			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionMissingDestinationService(otherServiceRef))
			t.Logf("reconciled to missing dest service: other")

			// Create the missing service, but forget the port.
			otherServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"other-"}},
				Ports: []*pbcatalog.ServicePort{{
					TargetPort: "http",
					Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
				}},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "other").
				WithData(t, otherServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs:     failoverData.PortConfigs,
				BoundReferences: []*pbresource.Reference{apiServiceRef, otherServiceRef},
			}
			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionUnknownDestinationPort(otherServiceRef, "admin"))
			t.Logf("reconciled to missing dest port other:admin")

			// fix the destination leg's port
			otherServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"other-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						VirtualPort: 8080,
						TargetPort:  "http",
						Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						VirtualPort: 10000,
						TargetPort:  "admin",
						Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "other").
				WithData(t, otherServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionOK)
			t.Logf("reconciled to accepted")

			// Update the two services to use differnet port names so the easy path doesn't work
			apiServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "foo",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "bar",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			otherServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"other-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "foo",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "baz",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "other").
				WithData(t, otherServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			failoverData = &pbcatalog.FailoverPolicy{
				Config: &pbcatalog.FailoverConfig{
					Destinations: []*pbcatalog.FailoverDestination{{
						Ref: otherServiceRef,
					}},
				},
			}
			failover = rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, failover.Id) })

			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"foo": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  otherServiceRef,
							Port: "foo",
						}},
					},
					"bar": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  otherServiceRef,
							Port: "bar",
						}},
					},
				},
				BoundReferences: []*pbresource.Reference{apiServiceRef, otherServiceRef},
			}
			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionUnknownDestinationPort(otherServiceRef, "bar"))
			t.Logf("reconciled to missing dest port other:bar")

			// and fix it the silly way by removing it from api+failover
			apiServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "foo",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			expectedComputedFP = &pbcatalog.ComputedFailoverPolicy{
				PortConfigs: map[string]*pbcatalog.FailoverConfig{
					"foo": {
						Destinations: []*pbcatalog.FailoverDestination{{
							Ref:  otherServiceRef,
							Port: "foo",
						}},
					},
				},
				BoundReferences: []*pbresource.Reference{apiServiceRef, otherServiceRef},
			}
			waitAndAssertComputedFailoverPolicy(t, client, failover.Id, expectedComputedFP, ConditionOK)
			t.Logf("reconciled to accepted")
		})
	}
}

func tenancySubTestName(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}

func waitAndAssertComputedFailoverPolicy(t *testing.T, client *rtest.Client, failoverId *pbresource.ID, expectedComputedFP *pbcatalog.ComputedFailoverPolicy, cond *pbresource.Condition) {
	cfpID := resource.ReplaceType(pbcatalog.ComputedFailoverPolicyType, failoverId)
	client.WaitForReconciliation(t, cfpID, ControllerID)
	client.WaitForStatusCondition(t, failoverId, ControllerID, cond)
	client.WaitForStatusCondition(t, cfpID, ControllerID, cond)
	client.WaitForResourceState(t, cfpID, func(t rtest.T, r *pbresource.Resource) {
		computedFp := client.RequireResourceExists(t, cfpID)
		decodedComputedFp := rtest.MustDecode[*pbcatalog.ComputedFailoverPolicy](t, computedFp)
		prototest.AssertDeepEqual(t, expectedComputedFP, decodedComputedFp.Data)
	})
}
