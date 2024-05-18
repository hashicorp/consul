// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package nodehealth

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/controller/cache"
	"github.com/hashicorp/consul/internal/controller/cache/indexers"
	"github.com/hashicorp/consul/internal/controller/dependency"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	nodeOwnerIndexName = "owner"
)

func NodeHealthController() *controller.Controller {
	return controller.NewController(StatusKey, pbcatalog.NodeType).
		WithWatch(pbcatalog.NodeHealthStatusType, dependency.MapOwnerFiltered(pbcatalog.NodeType), indexers.OwnerIndex(nodeOwnerIndexName)).
		WithReconciler(&nodeHealthReconciler{})
}

type nodeHealthReconciler struct{}

func (r *nodeHealthReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// The runtime is passed by value so replacing it here for the remainder of this
	// reconciliation request processing will not affect future invocations.
	rt.Logger = rt.Logger.With("resource-id", req.ID)

	rt.Logger.Trace("reconciling node health")

	// read the node
	node, err := rt.Cache.Get(pbcatalog.NodeType, "id", req.ID)
	if err != nil {
		rt.Logger.Error("the cache has returned an unexpected error", "error", err)
		return err
	}
	if node == nil {
		rt.Logger.Trace("node has been deleted")
		return nil
	}

	health, err := getNodeHealth(rt, req.ID)
	if err != nil {
		rt.Logger.Error("failed to calculate the nodes health", "error", err)
		return err
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: node.Generation,
		Conditions: []*pbresource.Condition{
			Conditions[health],
		},
	}

	if resource.EqualStatus(node.Status[StatusKey], newStatus, false) {
		rt.Logger.Trace("resources node health status is unchanged", "health", health.String())
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     node.Id,
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

func getNodeHealth(rt controller.Runtime, nodeRef *pbresource.ID) (pbcatalog.Health, error) {
	iter, err := cache.ListIteratorDecoded[*pbcatalog.NodeHealthStatus](rt.Cache, pbcatalog.NodeHealthStatusType, nodeOwnerIndexName, nodeRef)
	if err != nil {
		return pbcatalog.Health_HEALTH_CRITICAL, err
	}

	health := pbcatalog.Health_HEALTH_PASSING

	for hs, err := iter.Next(); hs != nil || err != nil; hs, err = iter.Next() {
		if err != nil {
			// This should be impossible as the resource service + type validations the
			// catalog is performing will ensure that no data gets written where unmarshalling
			// to this type will error.
			return pbcatalog.Health_HEALTH_CRITICAL, fmt.Errorf("error getting decoded health status data: %w", err)
		}

		if hs.Data.Status > health {
			health = hs.Data.Status
		}
	}

	return health, nil
}
