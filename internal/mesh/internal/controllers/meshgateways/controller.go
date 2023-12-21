// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package meshgateways

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	ControllerName = "consul.io/mesh-gateway"

	meshPortName = "mesh"
	wanPort      = 8443
	wanPortName  = "wan"
)

func Controller() *controller.Controller {
	r := &reconciler{}

	return controller.NewController(ControllerName, pbmesh.MeshGatewayType).
		WithReconciler(r)
}

type reconciler struct{}

// Reconcile is responsible for creating a Service w/ a MeshGateway owner,
// in addition to other things discussed in the RFC.
func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID)
	rt.Logger.Trace("reconciling mesh gateway")

	// TODO NET-6822 The ports and workload selector below are currently hardcoded
	//  until they are added to the MeshGateway resource and pulled from there.
	service := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{req.ID.Name},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				Protocol:    pbcatalog.Protocol_PROTOCOL_TCP,
				TargetPort:  wanPortName,
				VirtualPort: wanPort,
			},
			{
				Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
				TargetPort: meshPortName,
			},
		},
	}

	serviceData, err := anypb.New(service)
	if err != nil {
		return err
	}

	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Data:     serviceData,
			Id:       resource.ReplaceType(pbcatalog.ServiceType, req.ID),
			Metadata: map[string]string{"gateway-kind": "mesh-gateway"},
			Owner:    req.ID,
		},
	})
	if err != nil {
		return err
	}

	// TODO NET-6426, NET-6427, NET-6428, NET-6429, NET-6430, NET-6431, NET-6432
	return nil
}
