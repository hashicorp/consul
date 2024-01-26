// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package link

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hashicorp/consul/agent/hcp/bootstrap"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func cleanup(ctx context.Context, rt controller.Runtime, res *pbresource.Resource, dataDir string) error {
	rt.Logger.Trace("cleaning up link resource")
	if dataDir != "" {
		hcpConfigDir := filepath.Join(dataDir, bootstrap.SubDir)
		rt.Logger.Debug("deleting hcp-config dir", "dir", hcpConfigDir)
		err := os.RemoveAll(hcpConfigDir)
		if err != nil {
			return err
		}
	}

	err := ensureDeleted(ctx, rt, res)
	if err != nil {
		return err
	}

	return nil
}

func addFinalizer(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) error {
	// The statusKey doubles as the finalizer name for the link resource
	if resource.HasFinalizer(res, StatusKey) {
		rt.Logger.Trace("already has finalizer")
		return nil
	}

	// Finalizer hasn't been written, so add it.
	resource.AddFinalizer(res, StatusKey)
	_, err := rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: res})
	if err != nil {
		return err
	}
	rt.Logger.Trace("added finalizer")
	return err
}

// ensureDeleted makes sure a link is finally deleted
func ensureDeleted(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) error {
	// Remove finalizer if present
	if resource.HasFinalizer(res, StatusKey) {
		resource.RemoveFinalizer(res, StatusKey)
		_, err := rt.Client.Write(ctx, &pbresource.WriteRequest{Resource: res})
		if err != nil {
			return err
		}
		rt.Logger.Trace("removed finalizer")
	}

	// Finally, delete the link
	_, err := rt.Client.Delete(ctx, &pbresource.DeleteRequest{Id: res.Id})
	if err != nil {
		return err
	}

	// Success
	rt.Logger.Trace("finally deleted")
	return nil
}
