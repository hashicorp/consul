// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package namespace

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/tenancy/internal/controllers/common"
	"github.com/hashicorp/consul/proto-public/pbresource"
	pbtenancy "github.com/hashicorp/consul/proto-public/pbtenancy/v2beta1"
)

const (
	// StatusKey also serves as the finalizer name.
	StatusKey = "consul.io/namespace-controller"

	// Conditions and reasons are shared with partitions. See
	// common.go for the full list.
)

func Controller(registry resource.Registry) *controller.Controller {
	return controller.NewController(StatusKey, pbtenancy.NamespaceType).
		WithReconciler(&Reconciler{Registry: registry})
}

type Reconciler struct {
	Registry resource.Registry
}

// Reconcile is responsible for reconciling a namespace resource.
//
// When a namespace is created, ensures a finalizer is added for cleanup.
//
// When a namespace is marked for deletion, ensures tenants are deleted and
// the finalizer is removed.
func (r *Reconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	rt.Logger = rt.Logger.With("resource", req.ID.Name, "controller", "namespace")

	// Never reconcile the default namespace in the default partition since it is
	// created on system startup or snapshot restoration. Resource validation rules
	// protect them being deleted.
	if req.ID.Tenancy.Partition == resource.DefaultPartitionName && req.ID.Name == resource.DefaultNamespaceName {
		rt.Logger.Trace("skipping reconcile of default namespace")
		return nil
	}

	// Read namespace to make sure we have the latest version.
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		// Namespace deleted - nothing to do.
		rt.Logger.Trace("namespace not found, nothing to do")
		return nil
	case err != nil:
		rt.Logger.Error("failed read", "error", err)
		return err
	}
	res := rsp.Resource

	if resource.IsMarkedForDeletion(res) {
		return ensureDeleted(ctx, rt, r.Registry, res)
	}

	if err = common.EnsureHasFinalizer(ctx, rt, res, StatusKey); err != nil {
		return err
	}

	return common.WriteStatus(ctx, rt, res, StatusKey, common.ConditionAccepted, common.ReasonAcceptedOK, err)
}

func ensureDeleted(ctx context.Context, rt controller.Runtime, registry resource.Registry, res *pbresource.Resource) error {
	tenancy := &pbresource.Tenancy{
		Partition: res.Id.Tenancy.Partition,
		Namespace: res.Id.Name,
	}
	// Delete namespace scoped tenants
	if err := common.EnsureTenantsDeleted(ctx, rt, registry, res, resource.ScopeNamespace, tenancy); err != nil {
		rt.Logger.Error("failed deleting tenants", "error", err)
		return common.WriteStatus(ctx, rt, res, StatusKey, common.ConditionDeleted, common.ReasonDeletionInProgress, err)
	}

	// Delete namespace resource since all namespace scoped tenants are deleted
	if err := common.EnsureResourceDeleted(ctx, rt, res, StatusKey); err != nil {
		rt.Logger.Error("failed deleting namespace", "error", err)
		return err
	}
	return nil
}
