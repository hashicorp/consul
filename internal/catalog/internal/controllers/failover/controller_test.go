// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failover

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	svctest "github.com/hashicorp/consul/agent/grpc-external/services/resource/testing"
	"github.com/hashicorp/consul/internal/catalog/internal/mappers/failovermapper"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	rtest "github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/sdk/testutil"
)

type controllerSuite struct {
	suite.Suite

	ctx      context.Context
	client   *rtest.Client
	rt       controller.Runtime
	registry resource.Registry

	failoverMapper FailoverMapper

	ctl failoverPolicyReconciler
}

func (suite *controllerSuite) SetupTest() {
	suite.ctx = testutil.TestContext(suite.T())
	client, registry := svctest.RunResourceService2(suite.T(), types.Register)
	suite.registry = registry
	suite.rt = controller.Runtime{
		Client: client,
		Logger: testutil.Logger(suite.T()),
	}
	suite.client = rtest.NewClient(client)

	suite.failoverMapper = failovermapper.New()
}

func (suite *controllerSuite) TestController() {
	// This test's purpose is to exercise the controller in a halfway realistic
	// way, verifying the event triggers work in the live code.

	// Run the controller manager
	mgr := controller.NewManager(suite.client, suite.registry, suite.rt.Logger)
	mgr.Register(FailoverPolicyController(suite.failoverMapper))
	mgr.SetRaftLeader(true)
	go mgr.Run(suite.ctx)

	// Create an advance pointer to some services.
	apiServiceRef := resource.Reference(rtest.Resource(types.ServiceType, "api").ID(), "")
	otherServiceRef := resource.Reference(rtest.Resource(types.ServiceType, "other").ID(), "")

	// create a failover without any services
	failoverData := &pbcatalog.FailoverPolicy{
		Config: &pbcatalog.FailoverConfig{
			Destinations: []*pbcatalog.FailoverDestination{{
				Ref: apiServiceRef,
			}},
		},
	}
	failover := rtest.Resource(types.FailoverPolicyType, "api").
		WithData(suite.T(), failoverData).
		Write(suite.T(), suite.client)

	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionMissingService)

	// Provide the service.
	apiServiceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"api-"}},
		Ports: []*pbcatalog.ServicePort{{
			TargetPort: "http",
			Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
		}},
	}
	_ = rtest.Resource(types.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionOK)

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
	_ = rtest.Resource(types.FailoverPolicyType, "api").
		WithData(suite.T(), failoverData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionUnknownPort("admin"))

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
	_ = rtest.Resource(types.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionUsingMeshDestinationPort(apiServiceRef, "admin"))

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
	_ = rtest.Resource(types.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionOK)

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
	_ = rtest.Resource(types.FailoverPolicyType, "api").
		WithData(suite.T(), failoverData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionMissingDestinationService(otherServiceRef))

	// Create the missing service, but forget the port.
	otherServiceData := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{Prefixes: []string{"other-"}},
		Ports: []*pbcatalog.ServicePort{{
			TargetPort: "http",
			Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
		}},
	}
	_ = rtest.Resource(types.ServiceType, "other").
		WithData(suite.T(), otherServiceData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionUnknownDestinationPort(otherServiceRef, "admin"))

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
	_ = rtest.Resource(types.ServiceType, "other").
		WithData(suite.T(), otherServiceData).
		Write(suite.T(), suite.client)
	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionOK)

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
	_ = rtest.Resource(types.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)

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
	_ = rtest.Resource(types.ServiceType, "other").
		WithData(suite.T(), otherServiceData).
		Write(suite.T(), suite.client)

	failoverData = &pbcatalog.FailoverPolicy{
		Config: &pbcatalog.FailoverConfig{
			Destinations: []*pbcatalog.FailoverDestination{{
				Ref: otherServiceRef,
			}},
		},
	}
	failover = rtest.Resource(types.FailoverPolicyType, "api").
		WithData(suite.T(), failoverData).
		Write(suite.T(), suite.client)

	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionUnknownDestinationPort(otherServiceRef, "bar"))

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
	_ = rtest.Resource(types.ServiceType, "api").
		WithData(suite.T(), apiServiceData).
		Write(suite.T(), suite.client)

	suite.client.WaitForStatusCondition(suite.T(), failover.Id, StatusKey, ConditionOK)
}

func TestFailoverController(t *testing.T) {
	suite.Run(t, new(controllerSuite))
}
