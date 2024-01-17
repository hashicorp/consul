// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failover

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/controllertest"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestController(t *testing.T) {
	// This test's purpose is to exercise the controller in a halfway realistic
	// way, verifying the event triggers work in the live code.

	clientRaw := controllertest.NewControllerTestBuilder().
		WithTenancies(resourcetest.TestTenancies()...).
		WithResourceRegisterFns(types.Register, types.RegisterDNSPolicy).
		WithControllerRegisterFns(func(mgr *controller.Manager) {
			mgr.Register(FailoverPolicyController())
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

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionMissingService)
			t.Logf("reconciled to missing service status")

			// Provide the service.
			apiServiceData := &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{{
					TargetPort: "http",
					Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
				}},
			}
			svc := rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionOK)
			t.Logf("reconciled to accepted")

			// Update the failover to reference an unknown port
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
			svc = rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionUnknownPort("admin"))
			t.Logf("reconciled to unknown admin port")

			// update the service to fix the stray reference, but point to a mesh port
			apiServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "http",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "admin",
						Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionUsingMeshDestinationPort(apiServiceRef, "admin"))
			t.Logf("reconciled to using mesh destination port")

			// update the service to fix the stray reference to not be a mesh port
			apiServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "http",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "admin",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
				},
			}
			svc = rtest.Resource(pbcatalog.ServiceType, "api").
				WithData(t, apiServiceData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionOK)
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
			svc = rtest.Resource(pbcatalog.FailoverPolicyType, "api").
				WithData(t, failoverData).
				WithTenancy(tenancy).
				Write(t, client)

			t.Cleanup(func() { client.MustDelete(t, svc.Id) })

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionMissingDestinationService(otherServiceRef))
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

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionUnknownDestinationPort(otherServiceRef, "admin"))
			t.Logf("reconciled to missing dest port other:admin")

			// fix the destination leg's port
			otherServiceData = &pbcatalog.Service{
				Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"other-"}},
				Ports: []*pbcatalog.ServicePort{
					{
						TargetPort: "http",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
					},
					{
						TargetPort: "admin",
						Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
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

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionUnknownDestinationPort(otherServiceRef, "bar"))
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

			client.WaitForStatusCondition(t, failover.Id, ControllerID, ConditionOK)
			t.Logf("reconciled to accepted")
		})
	}
}

func tenancySubTestName(tenancy *pbresource.Tenancy) string {
	return fmt.Sprintf("%s_Namespace_%s_Partition", tenancy.Namespace, tenancy.Partition)
}
