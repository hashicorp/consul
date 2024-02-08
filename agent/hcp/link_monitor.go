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

// RunHCPLinkWatcher watches the HCP Link resource and kicks off a goroutine
// to manage the lifecycle of the HCP manager based on HCP Link events.
//
// StartLinkWatch will use the WatchList API in order to watch the HCP Link
// and return a channel which will contain WatchEvents.
// If there are any errors in doing so, it will continuously retry until successful.
//
// Once the channel is created, we call MonitorHCPLink which will runs continuously
// and handles starting/stopping the HCP manager.
func RunHCPLinkWatcher(
	ctx context.Context, logger hclog.Logger, client pbresource.ResourceServiceClient, m Manager,
	hcpClientFn func(cfg config.CloudConfig) (hcpclient.Client, error),
	loadMgmtTokenFn func(
		ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string,
	) (string, error),
	cloudConfig config.CloudConfig,
	dataDir string,
) {
	hcpLinkWatchCh := StartLinkWatch(
		ctx,
		logger,
		client,
	)

	MonitorHCPLink(ctx, logger, m, hcpLinkWatchCh, hcpClientFn, loadMgmtTokenFn, cloudConfig, dataDir)
}

// MonitorHCPLink monitors the status of the HCP Link and based on that, manages
// the lifecycle of the HCP Manager. It's designed to be run in its own goroutine
// for the life of a server agent. It should be run even if HCP is not configured
// yet for servers. When an HCP Link is created, it will Start the Manager and
// when an HCP Link is deleted, it will Stop the Manager.
func MonitorHCPLink(
	ctx context.Context,
	logger hclog.Logger,
	m Manager,
	hcpLinkEventCh chan *pbresource.WatchEvent,
	hcpClientFn func(cfg config.CloudConfig) (hcpclient.Client, error),
	loadMgmtTokenFn func(
		ctx context.Context, logger hclog.Logger, hcpClient hcpclient.Client, dataDir string,
	) (string, error),
	cloudConfig config.CloudConfig,
	dataDir string,
) {
	for watchEvent := range hcpLinkEventCh {
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
			continue
		}

		// This indicates that a Link was either created or updated
		if watchEvent.GetUpsert() != nil {
			logger.Debug("HCP Link upserted, starting manager if not already started")

			res := watchEvent.GetUpsert().GetResource()
			var link pbhcp.Link
			if err := res.GetData().UnmarshalTo(&link); err != nil {
				logger.Error("error unmarshalling link data", "error", err)
				continue
			}

			if validated, reason := hcpctl.IsValidated(res); !validated {
				logger.Debug("HCP Link not validated, not starting manager", "reason", reason)
				continue
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
				continue
			}

			// Load the management token if access is set to read-write. Read-only clusters
			// will not have a management token provided by HCP.
			var token string
			if link.GetAccessLevel() == pbhcp.AccessLevel_ACCESS_LEVEL_GLOBAL_READ_WRITE {
				token, err = loadMgmtTokenFn(ctx, logger, hcpClient, dataDir)
				if err != nil {
					logger.Error("error loading management token", "error", err)
					continue
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
