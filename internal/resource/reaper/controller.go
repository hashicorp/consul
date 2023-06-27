// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package reaper

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	statusKeyReaperController       = "consul.io/reaper-controller"
	secondPassDelay                 = 30 * time.Second
	conditionTypeFirstPassCompleted = "FirstPassCompleted"
)

// RegisterControllers registers controllers for the tombstone type.
func RegisterControllers(mgr *controller.Manager) {
	mgr.Register(reaperController())
}

func reaperController() controller.Controller {
	return controller.ForType(resource.TypeV1Tombstone).
		WithReconciler(newReconciler())
}

func newReconciler() *tombstoneReconciler {
	return &tombstoneReconciler{
		timeNow: time.Now,
	}
}

type tombstoneReconciler struct {
	// Testing shim
	timeNow func() time.Time
}

// Deletes all owned (child) resources of an owner (parent) resource.
//
// The reconciliation for tombstones is split into two passes.
// The first pass attempts to delete child resources created before the owner resource was deleted.
// The second pass is run after a reasonable delay to delete child resources that may have been
// created during or after the completion of the first pass.
func (r *tombstoneReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		// tombstone not found. nothing to do
		return nil
	case err != nil:
		// retry later
		return err
	}
	res := rsp.Resource

	var tombstone pbresource.Tombstone
	if err := res.Data.UnmarshalTo(&tombstone); err != nil {
		return err
	}

	firstPassCompletedOnEntry := isFirstPassCompleted(res)

	// Corner case:
	// Check secondPassDelay has elasped since first pass in cases where queued
	// reconciliation requests are lost between the first and second pass
	// (e.g. controller relocated to a different server due to raft leadership
	// change).
	if firstPassCompletedOnEntry && !r.secondPassDelayElapsed(res.Status[statusKeyReaperController]) {
		return controller.RequeueAfter(secondPassDelay)
	}

	// Retrieve owner's children
	listRsp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{Owner: tombstone.Owner})
	if err != nil {
		return err
	}

	// Attempt to delete each child
	for _, child := range listRsp.Resources {
		_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: child.Id})
		if err != nil {
			return err
		}
	}

	if firstPassCompletedOnEntry {
		// we just did the second pass -> delete tombstone
		_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
		if err != nil {
			// tombstone deletion failed, just retry
			return err
		}
		// tombstone delete succeeded and reconciliation complete
		return nil
	} else {
		// we just did the first pass -> queue up the second pass
		_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
			Id:  res.Id,
			Key: statusKeyReaperController,
			Status: &pbresource.Status{
				ObservedGeneration: res.Generation,
				Conditions: []*pbresource.Condition{
					{
						Type:    conditionTypeFirstPassCompleted,
						State:   pbresource.Condition_STATE_TRUE,
						Reason:  "Success",
						Message: "First pass of child resource deletion completed",
					},
				},
			},
		})
		if err != nil {
			return err
		}
		return controller.RequeueAfter(secondPassDelay)
	}
}

func (r *tombstoneReconciler) secondPassDelayElapsed(status *pbresource.Status) bool {
	firstPassTime := status.UpdatedAt.AsTime()
	return firstPassTime.Add(secondPassDelay).Before(r.timeNow())
}

func isFirstPassCompleted(res *pbresource.Resource) bool {
	if res.Status == nil {
		return false
	}

	status, ok := res.Status[statusKeyReaperController]
	if !ok {
		return false
	}

	// First time through, first and second pass ahead of us
	if len(status.Conditions) == 0 {
		return false
	}

	// Single condition "FirstPassCompleted"
	condition := status.Conditions[0]
	return condition.State == pbresource.Condition_STATE_TRUE
}
