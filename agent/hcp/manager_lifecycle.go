// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"os"
	"path/filepath"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/hcp/bootstrap/constants"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/agent/hcp/config"
	hcpctl "github.com/hashicorp/consul/internal/hcp"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// HCPManagerLifecycleFn returns a LinkEventHandler function which will appropriately
// Start and Stop the HCP Manager based on the Link event received. If a link is upserted,
// the HCP Manager is started, and if a link is deleted, the HCP manager is stopped.
func HCPManagerLifecycleFn(
	m Manager,
	hcpClientFn func(cfg config.CloudConfig) (hcpclient.Client, error),
	loadMgmtTokenFn func(
		ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string,
	) (string, error),
	cloudConfig config.CloudConfig,
	dataDir string,
) LinkEventHandler {
	return func(ctx context.Context, logger hclog.Logger, watchEvent *pbresource.WatchEvent) {
		// This indicates that a Link was deleted
		if watchEvent.GetDelete() != nil {
			logger.Debug("HCP Link deleted, stopping HCP manager")

			if dataDir != "" {
				hcpConfigDir := filepath.Join(dataDir, constants.SubDir)
				logger.Debug("deleting hcp-config dir", "dir", hcpConfigDir)
				err := os.RemoveAll(hcpConfigDir)
				if err != nil {
					logger.Error("failed to delete hcp-config dir", "dir", hcpConfigDir, "err", err)
				}
			}

			err := m.Stop()
			if err != nil {
				logger.Error("error stopping HCP manager", "error", err)
			}
			return
		}

		// This indicates that a Link was either created or updated
		if watchEvent.GetUpsert() != nil {
			logger.Debug("HCP Link upserted, starting manager if not already started")

			res := watchEvent.GetUpsert().GetResource()
			var link pbhcp.Link
			if err := res.GetData().UnmarshalTo(&link); err != nil {
				logger.Error("error unmarshalling link data", "error", err)
				return
			}

			if validated, reason := hcpctl.IsValidated(res); !validated {
				logger.Debug("HCP Link not validated, not starting manager", "reason", reason)
				return
			}

			// Update the HCP manager configuration with the link values
			// Merge the link data with the existing cloud config so that we only overwrite the
			// fields that are provided by the link. This ensures that:
			// 1. The HCP configuration (i.e., how to connect to HCP) is preserved
			// 2. The Consul agent's node ID and node name are preserved
			newCfg := config.CloudConfig{
				ResourceID:   link.ResourceId,
				ClientID:     link.ClientId,
				ClientSecret: link.ClientSecret,
			}
			mergedCfg := config.Merge(cloudConfig, newCfg)
			hcpClient, err := hcpClientFn(mergedCfg)
			if err != nil {
				logger.Error("error creating HCP client", "error", err)
				return
			}

			// Load the management token if access is set to read-write. Read-only clusters
			// will not have a management token provided by HCP.
			var token string
			if link.GetAccessLevel() == pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE {
				token, err = loadMgmtTokenFn(ctx, logger, hcpClient, dataDir)
				if err != nil {
					logger.Error("error loading management token", "error", err)
					return
				}
			}

			mergedCfg.ManagementToken = token
			m.UpdateConfig(hcpClient, mergedCfg)

			err = m.Start(ctx)
			if err != nil {
				logger.Error("error starting HCP manager", "error", err)
			}
		}
	}
}
