// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Common code shared by the partition and namespace controllers.
package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

const (
	// ConditionAccepted indicates the tenancy unit has a finalizer
	// and contains a default namespace if a partition.
	ConditionAccepted              = "accepted"
	ReasonAcceptedOK               = "Ok"
	ReasonEnsureHasFinalizerFailed = "EnsureHasFinalizerFailed"

	// ConditionDeleted indicates that the units tenants have been
	// deleted. It never has a state other than false because the
	// resource no longer exists at that point.
	ConditionDeleted         = "deleted"
	ReasonDeletionInProgress = "DeletionInProgress"
)

var (
	ErrStillHasTenants = errors.New("still has tenants")
)

func EnsureHasFinalizer(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, statusKey string) error {
	// The statusKey doubles as the finalizer name for tenancy resources.
	if resource.HasFinalizer(res, statusKey) {
		rt.Logger.Trace("already has finalizer")
		return nil
	}

	// Finalizer hasn't been written, so add it.
	resource.AddFinalizer(res, statusKey)
	_, err := rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	if err != nil {
		return WriteStatus(ctx, rt, res, statusKey, ConditionAccepted, ReasonEnsureHasFinalizerFailed, err)
	}
	rt.Logger.Trace("added finalizer")
	return err
}

func EnsureTenantsDeleted(ctx context.Context, rt controller.Runtime, registry resource.Registry, res *pbresource.Resource, tenantScope resource.Scope, tenancy *pbresource.Tenancy) error {
	// Useful stats to keep track of on every sweep
	numExistingHasFinalizer := 0
	numExistingOwned := 0
	numImmediateDeletes := 0
	numDeferredDeletes := 0

	// List doesn't support querying across all types so iterate through each one.
	for _, reg := range registry.Types() {
		// Skip tenants that aren't scoped to the tenancy unit.
		if reg.Scope != tenantScope {
			continue
		}

		// Get all tenants of the current type.
		rsp, err := rt.Client.List(ctx, &pbresource.ListRequest{Type: reg.Type, Tenancy: tenancy})
		if err != nil {
			return err
		}

		if len(rsp.Resources) > 0 {
			rt.Logger.Trace(fmt.Sprintf("found %d tenant %s", len(rsp.Resources), reg.Type.Kind))
		}

		// Delete each qualified tenant.
		for _, tenant := range rsp.Resources {
			// Owned resources will be deleted when the parent resource is deleted (tombstone reaper)
			// so just skip over them.
			if tenant.Owner != nil {
				numExistingOwned++
				continue
			}

			// Skip anything that is already marked for deletion and has finalizers
			// since deletion of those resource is out of our control.
			if resource.IsMarkedForDeletion(tenant) && resource.HasFinalizers(tenant) {
				numExistingHasFinalizer++
				continue
			}

			// Delete tenant with a blanket non-CAS delete since we don't care about the version that
			// is deleted. Since we don't know whether the delete was immediate or deferred due to the
			// presense of a finalizer, we can't assume that the tenant is really deleted.
			_, err = rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: tenant.Id, Version: ""})
			if err != nil {
				// Bail on the first sign of trouble and retry in future reconciles.
				return err
			}

			// Classify the just deleted tenant since we're not fully vacated if a deferred delete occurred.
			if resource.HasFinalizers(tenant) {
				rt.Logger.Trace(fmt.Sprintf("deferred delete of %s tenant %q", res.Id.Type.Kind, tenant.Id.Name))
				numDeferredDeletes++
			} else {
				rt.Logger.Trace(fmt.Sprintf("immediate delete of %s tenant %q", res.Id.Type.Kind, tenant.Id.Name))
				numImmediateDeletes++
			}
		}
	}

	// Force re-reconcile if we have any lingering tenants by returning an error.
	if numExistingOwned+numExistingHasFinalizer+numDeferredDeletes > 0 {
		if numExistingOwned > 0 {
			rt.Logger.Debug(fmt.Sprintf("delete blocked on %d remaining owned tenants", numExistingOwned))
		}
		if numExistingHasFinalizer > 0 {
			rt.Logger.Debug(fmt.Sprintf("delete blocked on %d remaining tenants with finalizers", numExistingHasFinalizer))
		}
		if numDeferredDeletes > 0 {
			rt.Logger.Debug(fmt.Sprintf("delete blocked on %d tenants which were just marked for deletion", numDeferredDeletes))
		}
		return ErrStillHasTenants
	}

	// We should have zero tenants and be good to continue.
	rt.Logger.Debug("no tenants - green light the delete")
	return nil
}

// EnsureResourceDelete makes sure a tenancy unit (partition or namespace) with no tenants is finally deleted.
func EnsureResourceDeleted(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, statusKey string) error {
	// Remove finalizer if present
	if resource.HasFinalizer(res, statusKey) {
		resource.RemoveFinalizer(res, statusKey)
		_, err := rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: res})
		if err != nil {
			rt.Logger.Error("failed write to remove finalizer")
			return WriteStatus(ctx, rt, res, statusKey, ConditionDeleted, ReasonDeletionInProgress, err)
		}
		rt.Logger.Trace("removed finalizer")
	}

	// Finally, delete the tenancy unit.
	_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
	if err != nil {
		rt.Logger.Error("failed final delete", "error", err)
		return WriteStatus(ctx, rt, res, statusKey, ConditionDeleted, ReasonDeletionInProgress, err)
	}

	// Success
	rt.Logger.Trace("finally deleted")
	return nil
}

// WriteStatus writes the tenancy resource status only if the status has changed.
// The state and message are based on whether the passed in error is nil
// (state=TRUE, message="") or not (state=FALSE, message=error). The passed in
// error is always returned unless the delegated call to client.WriteStatus
// itself fails.
func WriteStatus(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, statusKey string, condition string, reason string, err error) error {
	state := pbresource.Condition_STATE_TRUE
	message := ""
	if err != nil {
		state = pbresource.Condition_STATE_FALSE
		message = err.Error()
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{{
			Type:    condition,
			State:   state,
			Reason:  reason,
			Message: message,
		}},
	}

	// Skip the write if the status hasn't changed to keep write amplificiation in check.
	if resource.EqualStatus(res.Status[statusKey], newStatus, false) {
		return err
	}

	_, statusErr := rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    statusKey,
		Status: newStatus,
	})

	if statusErr != nil {
		rt.Logger.Error("failed writing status", "error", statusErr)
		return statusErr
	}
	rt.Logger.Trace("wrote status", "status", newStatus)
	return err
}
