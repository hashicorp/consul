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
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)
	rt.Logger.Trace("reconciling mesh gateway")

	service := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			// Assumes that our MeshGateway Deployment in K8s is named the same as the MeshGateway resource in Consul
			// TODO(nathancoleman) Fetch the MeshGateway and use WorkloadSelector from there
			Prefixes: []string{req.ID.Name},
		},
	}

	serviceData, err := anypb.New(service)
	if err != nil {
		return err
	}

	_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
		Resource: &pbresource.Resource{
			Id:    resource.ReplaceType(pbcatalog.ServiceType, req.ID),
			Owner: req.ID,
			Data:  serviceData,
		},
	})
	if err != nil {
		return err
	}

	// TODO NET-6426, NET-6427, NET-6428, NET-6429, NET-6430, NET-6431, NET-6432
	return nil
}
