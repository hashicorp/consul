// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package mesh

import (
	"context"

	"github.com/hashicorp/consul/envoyextensions/xdscommon"
	"github.com/hashicorp/consul/internal/catalog"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/mesh/builder"
	"github.com/hashicorp/consul/internal/mesh/internal/controllers/mesh/mappers"
	"github.com/hashicorp/consul/internal/mesh/internal/types"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ControllerName is the name for this controller. It's used for logging or status keys.
const ControllerName = "consul.io/mesh-controller"

func Controller() controller.Controller {
	return controller.ForType(types.ProxyStateTemplateType).
		WithWatch(catalog.ServiceEndpointsType, mappers.MapServiceEndpointsToProxyStateTemplate).
		WithReconciler(&reconciler{})
}

type reconciler struct{}

func (r *reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource-id", req.ID, "controller", ControllerName)

	rt.Logger.Trace("reconciling proxy state template", "id", req.ID)

	// Check if the workload exists.
	workloadID := workloadIDFromProxyStateTemplate(req.ID)
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: workloadID})

	switch {
	case status.Code(err) == codes.NotFound:
		// If workload has been deleted, then return as ProxyStateTemplate should be cleaned up
		// by the garbage collector because of the owner reference.
		rt.Logger.Trace("workload doesn't exist; skipping reconciliation", "workload", workloadID)
		return nil
	case err != nil:
		rt.Logger.Error("error reading the associated workload", "error", err)
		return err
	}

	// Parse the workload data for this proxy. Note that we know that this workload has a service associated with it
	// because we only trigger updates off of service endpoints.
	workloadRes := rsp.Resource
	var workload pbcatalog.Workload
	err = workloadRes.Data.UnmarshalTo(&workload)
	if err != nil {
		rt.Logger.Error("error parsing workload data", "workload", workloadRes.Id)
		return resource.NewErrDataParse(&workload, err)
	}

	rsp, err = rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	var buildNew bool
	switch {
	case status.Code(err) == codes.NotFound:
		// Nothing to do as this resource may not have been created yet.
		rt.Logger.Trace("proxy state template for this workload doesn't yet exist; generating a new one", "id", req.ID)
		buildNew = true
	case err != nil:
		rt.Logger.Error("error reading proxy state template", "error", err)
		return nil
	}

	if !isMeshEnabled(workload.Ports) {
		// Skip non-mesh workloads.

		// If there's existing proxy state template, delete it.
		if !buildNew {
			rt.Logger.Trace("deleting existing proxy state template because workload is no longer on the mesh", "id", req.ID)
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: req.ID})
			if err != nil {
				rt.Logger.Error("error deleting existing proxy state template", "error", err)
				return err
			}
		}
		rt.Logger.Trace("skipping proxy state template generation because workload is not on the mesh", "workload", workloadRes.Id)
		return nil
	}

	var proxyTemplate pbmesh.ProxyStateTemplate
	if !buildNew {
		err = rsp.Resource.Data.UnmarshalTo(&proxyTemplate)
		if err != nil {
			rt.Logger.Error("error parsing proxy state template data", "id", req.ID)
			return resource.NewErrDataParse(&proxyTemplate, err)
		}
	}

	b := builder.New(req.ID, workloadIdentityRefFromWorkload(workloadRes.Id)).
		AddInboundListener(xdscommon.PublicListenerName, &workload).
		AddInboundRouters(&workload).
		AddInboundTLS()

	newProxyTemplate := b.Build()

	same := proto.Equal(&proxyTemplate, newProxyTemplate)
	if buildNew || !same {
		proxyTemplateData, err := anypb.New(newProxyTemplate)
		if err != nil {
			rt.Logger.Error("error creating proxy state template data", "error", err)
			return err
		}
		rt.Logger.Trace("updating proxy state template", "id", req.ID)
		_, err = rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:    req.ID,
				Owner: workloadRes.Id,
				Data:  proxyTemplateData,
			},
		})
		if err != nil {
			rt.Logger.Error("error writing proxy state template", "error", err)
			return err
		}
	} else {
		rt.Logger.Trace("proxy state template data has not changed, skipping update", "id", req.ID)
	}
	return nil
}

func workloadIDFromProxyStateTemplate(id *pbresource.ID) *pbresource.ID {
	return &pbresource.ID{
		Name:    id.Name,
		Tenancy: id.Tenancy,
		Type:    catalog.WorkloadType,
	}
}

func workloadIdentityRefFromWorkload(id *pbresource.ID) *pbresource.Reference {
	return &pbresource.Reference{
		Name:    id.Name,
		Tenancy: id.Tenancy,
	}
}

// isMeshEnabled returns true if workload or service endpoints port
// contain a port with the "mesh" protocol.
func isMeshEnabled(ports map[string]*pbcatalog.WorkloadPort) bool {
	for _, port := range ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			return true
		}
	}
	return false
}
