// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodehealth

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func NodeHealthController() controller.Controller {
	return controller.ForType(types.NodeType).
		WithWatch(types.HealthStatusType, controller.MapOwnerFiltered(types.NodeType)).
		WithReconciler(&nodeHealthReconciler{})
}

type nodeHealthReconciler struct{}

func (r *nodeHealthReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID)

	rt.Logger.Trace("reconciling node health")

	// read the node
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		rt.Logger.Trace("node has been deleted")
		return nil
	case err != nil:
		rt.Logger.Error("the resource service has returned an unexpected error", "error", err)
		return err
	}

	res := rsp.Resource

	health, err := getNodeHealth(ctx, rt, req.ID)
	if err != nil {
		rt.Logger.Error("failed to calculate the nodes health", "error", err)
		return err
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{
			Conditions[health],
		},
	}

	if resource.EqualStatus(res.Status[StatusKey], newStatus, false) {
		rt.Logger.Trace("resources node health status is unchanged", "health", health.String())
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    StatusKey,
		Status: newStatus,
	})

	if err != nil {
		rt.Logger.Error("error encountered when attempting to update the resources node health status", "error", err)
		return err
	}

	rt.Logger.Trace("resources node health status was updated", "health", health.String())
	return nil
}

func getNodeHealth(ctx context.Context, rt controller.Runtime, nodeRef *pbresource.ID) (pbcatalog.Health, error) {
	rsp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{
		Owner: nodeRef,
	})

	if err != nil {
		return pbcatalog.Health_HEALTH_CRITICAL, err
	}

	health := pbcatalog.Health_HEALTH_PASSING

	for _, res := range rsp.Resources {
		if resource.EqualType(res.Id.Type, types.HealthStatusType) {
			var hs pbcatalog.HealthStatus
			if err := res.Data.UnmarshalTo(&hs); err != nil {
				// This should be impossible as the resource service + type validations the
				// catalog is performing will ensure that no data gets written where unmarshalling
				// to this type will error.
				return pbcatalog.Health_HEALTH_CRITICAL, fmt.Errorf("error unmarshalling health status data: %w", err)
			}

			if hs.Status > health {
				health = hs.Status
			}
		}
	}

	return health, nil
}
