// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

// MonitorHCPLink monitors the status of the HCP Link and based on that, manages
// the lifecycle of the HCP Manager. It's designed to be run in its own goroutine
// for the life of a server agent. It should be run even if HCP is not configured
// yet for servers. When an HCP Link is created, it will Start the Manager and
// when an HCP Link is deleted, it will Stop the Manager.
func MonitorHCPLink(ctx context.Context, logger hclog.Logger, hcpLinkEventCh chan *pbresource.WatchEvent, m Manager) {
	for {
		watchEvent := <-hcpLinkEventCh

		if watchEvent.GetDelete() != nil {
			logger.Debug("HCP Link deleted, stopping HCP manager")
			err := m.Stop()
			if err != nil {
				logger.Error("error stopping HCP manager", "error", err)
			}
			continue
		}

		err := m.Start(ctx)
		if err != nil {
			logger.Error("error starting HCP manager", "error", err)
		}
	}
}
