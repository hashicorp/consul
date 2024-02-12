// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package failover

import (
	"context"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/failover/expander"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	pbmulticluster "github.com/hashicorp/consul/proto-public/pbmulticluster/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	destRefsIndexName  = "destination-refs"
	boundRefsIndexName = "bound-refs"
)

func FailoverPolicyController(sgExpander expander.SamenessGroupExpander) *controller.Controller {
	ctrl := controller.NewController(
		ControllerID,
		pbcatalog.ComputedFailoverPolicyType,
		indexers.BoundRefsIndex[*pbcatalog.ComputedFailoverPolicy](boundRefsIndexName),
	).
		WithWatch(
			pbcatalog.ServiceType,
			dependency.MultiMapper(
				// FailoverPolicy is name-aligned with the Service it controls so always
				// re-reconcile the corresponding FailoverPolicy when a Service changes.
				dependency.ReplaceType(pbcatalog.ComputedFailoverPolicyType),
				dependency.WrapAndReplaceType(
					pbcatalog.ComputedFailoverPolicyType,
					dependency.CacheParentsMapper(pbcatalog.ComputedFailoverPolicyType, boundRefsIndexName),
				),
			),
		).
		WithWatch(
			pbcatalog.FailoverPolicyType,
			dependency.ReplaceType(pbcatalog.ComputedFailoverPolicyType),
			sgExpander.GetSamenessGroupIndex(),
		).
		WithReconciler(newFailoverPolicyReconciler(sgExpander))

	return registerEnterpriseControllerWatchers(ctrl)
}

type failoverPolicyReconciler struct {
	sgExpander expander.SamenessGroupExpander
}

func newFailoverPolicyReconciler(sgExpander expander.SamenessGroupExpander) *failoverPolicyReconciler {
	return &failoverPolicyReconciler{
		sgExpander: sgExpander,
	}
}

func (r *failoverPolicyReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID)

	rt.Logger.Trace("reconciling computed failover policy")

	computedFailoverPolicy, err := cache.GetDecoded[*pbcatalog.ComputedFailoverPolicy](rt.Cache, pbcatalog.ComputedFailoverPolicyType, "id", req.ID)
	if err != nil {
		rt.Logger.Error("error retrieving computed failover policy", "error", err)
		return err
	}
	failoverPolicyID := resource.ReplaceType(pbcatalog.FailoverPolicyType, req.ID)
	failoverPolicy, err := cache.GetDecoded[*pbcatalog.FailoverPolicy](rt.Cache, pbcatalog.FailoverPolicyType, "id", failoverPolicyID)
	if err != nil {
		rt.Logger.Error("error retrieving failover policy", "error", err)
		return err
	}
	if failoverPolicy == nil {
		if err := deleteResource(ctx, rt, computedFailoverPolicy.GetResource()); err != nil {
			rt.Logger.Error("failed to delete computed failover policy", "error", err)
			return err
		}

		return nil
	}
	// Capture original raw config for pre-normalization status conditions.
	rawFailoverPolicy := failoverPolicy.Data

	// FailoverPolicy is name-aligned with the Service it controls.
	serviceID := &pbresource.ID{
		Type:    pbcatalog.ServiceType,
		Tenancy: failoverPolicyID.Tenancy,
		Name:    failoverPolicyID.Name,
	}

	service, err := cache.GetDecoded[*pbcatalog.Service](rt.Cache, pbcatalog.ServiceType, "id", serviceID)
	if err != nil {
		rt.Logger.Error("error retrieving corresponding service", "error", err)
		return err
	}

	if service == nil {
		if err := deleteResource(ctx, rt, computedFailoverPolicy.GetResource()); err != nil {
			rt.Logger.Error("failed to delete computed failover policy", "error", err)
			return err
		}

		conds := []*pbresource.Condition{ConditionMissingService}

		if err := writeStatus(ctx, rt, failoverPolicy.Resource, conds); err != nil {
			rt.Logger.Error("error encountered when attempting to update the resource's failover policy status", "error", err)
			return err
		}
		rt.Logger.Trace("resource's failover policy status was updated",
			"conditions", conds)
		return nil
	}

	newComputedFailoverPolicy, destServices, missingSamenessGroups, err := makeComputedFailoverPolicy(ctx, rt, r.sgExpander, failoverPolicy, service)
	if err != nil {
		return err
	}
	computedFailoverResource := computedFailoverPolicy.GetResource()

	if !proto.Equal(computedFailoverPolicy.GetData(), newComputedFailoverPolicy) {

		newCFPData, err := anypb.New(newComputedFailoverPolicy)
		if err != nil {
			rt.Logger.Error("error marshalling new computed failover policy", "error", err)
			return err
		}
		rt.Logger.Trace("writing computed failover policy")
		rsp, err := rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:   req.ID,
				Data: newCFPData,
			},
		})
		if err != nil || rsp.Resource == nil {
			rt.Logger.Error("error writing new computed failover policy", "error", err)
			return err
		} else {
			rt.Logger.Trace("new computed failover policy was successfully written")
			computedFailoverResource = rsp.Resource
		}
	}

	conds := computeNewConditions(rawFailoverPolicy, failoverPolicy.Resource, newComputedFailoverPolicy, service, destServices, missingSamenessGroups)
	if err := writeStatus(ctx, rt, failoverPolicy.Resource, conds); err != nil {
		rt.Logger.Error("error encountered when attempting to update the resource's failover policy status", "error", err)
		return err
	}

	conds = computeNewConditions(rawFailoverPolicy, computedFailoverResource, newComputedFailoverPolicy, service, destServices, missingSamenessGroups)
	if err := writeStatus(ctx, rt, computedFailoverResource, conds); err != nil {
		rt.Logger.Error("error encountered when attempting to update the resource's computed failover policy status", "error", err)
		return err
	}

	return nil
}

func computeNewConditions(
	rawFailoverPolicy *pbcatalog.FailoverPolicy,
	fpRes *pbresource.Resource,
	fp *pbcatalog.ComputedFailoverPolicy,
	service *resource.DecodedResource[*pbcatalog.Service],
	destServices map[resource.ReferenceKey]*resource.DecodedResource[*pbcatalog.Service],
	missingSamenessGroups map[string]struct{},
) []*pbresource.Condition {

	allowedPortProtocols := make(map[string]pbcatalog.Protocol)
	for _, port := range service.Data.Ports {
		if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
			continue // skip
		}
		allowedPortProtocols[port.TargetPort] = port.Protocol
	}

	var conditions []*pbresource.Condition

	if rawFailoverPolicy != nil {
		// We need to validate port mappings on the raw input config due to the
		// possibility of duplicate mappings, which will be normalized into one
		// mapping by target port key.
		usedTargetPorts := make(map[string]any)
		for port := range rawFailoverPolicy.PortConfigs {
			svcPort := service.Data.FindPortByID(port)
			targetPort := svcPort.GetTargetPort() // svcPort could be nil

			serviceRef := resource.NewReferenceKey(service.Id).ToReference()
			if svcPort == nil {
				conditions = append(conditions, ConditionUnknownPort(serviceRef, port))
			} else if _, ok := usedTargetPorts[targetPort]; ok {
				conditions = append(conditions, ConditionConflictDestinationPort(serviceRef, svcPort))
			} else {
				usedTargetPorts[targetPort] = struct{}{}
			}
		}
	}

	for _, pc := range fp.GetPortConfigs() {
		for _, dest := range pc.Destinations {
			// We know from validation that a Ref must be set, and the type it
			// points to is a Service.
			//
			// Rather than do additional validation, just do a quick
			// belt-and-suspenders check-and-skip if something looks weird.
			if dest.Ref == nil || !isServiceType(dest.Ref.Type) {
				continue
			}

			if cond := serviceHasPort(dest, destServices); cond != nil {
				conditions = append(conditions, cond)
			}
		}
	}

	for destKey, svc := range destServices {
		if svc != nil {
			continue
		}
		conditions = append(conditions, ConditionMissingDestinationService(destKey.ToReference()))
	}

	for sg := range missingSamenessGroups {
		ref := &pbresource.Reference{
			Type: pbmulticluster.SamenessGroupType,
			Tenancy: &pbresource.Tenancy{
				Partition: fpRes.GetId().GetTenancy().GetPartition(),
			},
			Name: sg,
		}
		conditions = append(conditions, ConditionMissingSamenessGroup(ref))
	}

	return conditions
}

func serviceHasPort(
	dest *pbcatalog.FailoverDestination,
	destServices map[resource.ReferenceKey]*resource.DecodedResource[*pbcatalog.Service],
) *pbresource.Condition {
	key := resource.NewReferenceKey(dest.Ref)
	destService, ok := destServices[key]
	if !ok || destService == nil {
		return nil
	}

	found := false
	mesh := false
	for _, port := range destService.Data.Ports {
		if port.TargetPort == dest.Port {
			found = true
			if port.Protocol == pbcatalog.Protocol_PROTOCOL_MESH {
				mesh = true
			}
			break
		}
	}

	if !found {
		return ConditionUnknownDestinationPort(dest.Ref, dest.Port)
	} else if mesh {
		return ConditionUsingMeshDestinationPort(dest.Ref, dest.Port)
	}

	return nil
}

func isServiceType(typ *pbresource.Type) bool {
	switch {
	case resource.EqualType(typ, pbcatalog.ServiceType):
		return true
	}
	return false
}

func deleteResource(ctx context.Context, rt controller.Runtime, resource *pbresource.Resource) error {
	if resource == nil {
		return nil
	}
	_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{
		Id:      resource.GetId(),
		Version: resource.GetVersion(),
	})
	if err != nil {
		return err
	}
	return nil
}

func makeComputedFailoverPolicy(ctx context.Context, rt controller.Runtime, sgExpander expander.SamenessGroupExpander, failoverPolicy *resource.DecodedResource[*pbcatalog.FailoverPolicy], service *resource.DecodedResource[*pbcatalog.Service]) (*pbcatalog.ComputedFailoverPolicy, map[resource.ReferenceKey]*resource.DecodedResource[*pbcatalog.Service], map[string]struct{}, error) {
	simplified := types.SimplifyFailoverPolicy(
		service.Data,
		failoverPolicy.Data,
	)
	cfp := &pbcatalog.ComputedFailoverPolicy{

		PortConfigs: simplified.PortConfigs,
	}
	missingSamenessGroups := make(map[string]struct{})
	destServices := map[resource.ReferenceKey]*resource.DecodedResource[*pbcatalog.Service]{
		resource.NewReferenceKey(service.Id): service,
	}

	// Expand sameness group
	for port, fc := range cfp.PortConfigs {
		if fc.GetSamenessGroup() == "" {
			continue
		}

		dests, missing, err := sgExpander.ComputeFailoverDestinationsFromSamenessGroup(rt, failoverPolicy.Id, fc.GetSamenessGroup(), port)
		if err != nil {
			return cfp, nil, missingSamenessGroups, err
		}

		if missing != "" {
			delete(cfp.PortConfigs, port)
			missingSamenessGroups[missing] = struct{}{}
			continue
		}

		if len(dests) == 0 {
			delete(cfp.PortConfigs, port)
			continue
		}

		fc.SamenessGroup = ""
		fc.Destinations = dests
	}

	// Filter missing destinations
	for port, fc := range cfp.PortConfigs {
		if len(fc.Destinations) == 0 {
			continue
		}

		var err error
		fc.Destinations, err = filterInvalidDests(ctx, rt, fc.Destinations, destServices)
		if err != nil {
			return nil, nil, nil, err
		}

		if len(fc.GetDestinations()) == 0 {
			delete(cfp.GetPortConfigs(), port)

		}
	}

	for ref := range destServices {
		cfp.BoundReferences = append(cfp.BoundReferences, ref.ToReference())
	}

	return cfp, destServices, missingSamenessGroups, nil
}

func filterInvalidDests(ctx context.Context, rt controller.Runtime, dests []*pbcatalog.FailoverDestination, destServices map[resource.ReferenceKey]*resource.DecodedResource[*pbcatalog.Service]) ([]*pbcatalog.FailoverDestination, error) {
	var out []*pbcatalog.FailoverDestination
	for _, dest := range dests {
		ref := resource.NewReferenceKey(dest.Ref)
		if svc, ok := destServices[ref]; ok {
			if svc != nil {
				out = append(out, dest)
			}
			continue
		}

		destService, err := resource.GetDecodedResource[*pbcatalog.Service](ctx, rt.Client, resource.IDFromReference(dest.Ref))
		if err != nil {
			rt.Logger.Error("error retrieving destination service while filtering", "service", dest, "error", err)
			return nil, err
		}
		if destService != nil {
			out = append(out, dest)
		}
		destServices[resource.NewReferenceKey(dest.Ref)] = destService
	}
	return out, nil
}

func writeStatus(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, conditions []*pbresource.Condition) error {
	newStatus := &pbresource.Status{
		ObservedGeneration: res.GetGeneration(),
		Conditions: []*pbresource.Condition{
			ConditionOK,
		},
	}

	if len(conditions) > 0 {
		newStatus = &pbresource.Status{
			ObservedGeneration: res.GetGeneration(),
			Conditions:         conditions,
		}
	}

	if !resource.EqualStatus(res.GetStatus()[ControllerID], newStatus, false) {

		_, err := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
			Id:     res.Id,
			Key:    ControllerID,
			Status: newStatus,
		})

		if err != nil {
			return err
		}
		rt.Logger.Trace("resource's status was updated",
			"conditions", newStatus.Conditions)

	}
	return nil
}
