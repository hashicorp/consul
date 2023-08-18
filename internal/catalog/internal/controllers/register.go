// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package controllers

import (
	"github.com/hashicorp/consul/internal/auth/internal/controllers/trafficpermissions"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/endpoints"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/failover"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/controller"
)

type Dependencies struct {
	WorkloadHealthNodeMapper workloadhealth.NodeMapper
	EndpointsWorkloadMapper  endpoints.WorkloadMapper
	FailoverMapper           failover.FailoverMapper
	TrafficPermissionsMapper trafficpermissions.Mapper
}

func Register(mgr *controller.Manager, deps Dependencies) {
	mgr.Register(nodehealth.NodeHealthController())
	mgr.Register(workloadhealth.WorkloadHealthController(deps.WorkloadHealthNodeMapper))
	mgr.Register(endpoints.ServiceEndpointsController(deps.EndpointsWorkloadMapper))
	mgr.Register(failover.FailoverPolicyController(deps.FailoverMapper))
	mgr.Register(trafficpermissions.Controller(deps.TrafficPermissionsMapper))
}
