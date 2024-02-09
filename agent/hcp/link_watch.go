// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"time"

	"github.com/hashicorp/go-hclog"

	hcpctl "github.com/hashicorp/consul/internal/hcp"
	"github.com/hashicorp/consul/lib/retry"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

type LinkEventHandler = func(context.Context, hclog.Logger, *pbresource.WatchEvent)

func handleLinkEvents(ctx context.Context, logger hclog.Logger, watchClient pbresource.ResourceService_WatchListClient, linkEventHandler LinkEventHandler) {
	for {
		select {
		case <-ctx.Done():
			logger.Debug("context canceled, exiting")
			return
		default:
			watchEvent, err := watchClient.Recv()

			if err != nil {
				logger.Error("error receiving link watch event", "error", err)
				return
			}

			linkEventHandler(ctx, logger, watchEvent)
		}
	}
}

func RunHCPLinkWatcher(
	ctx context.Context, logger hclog.Logger, client pbresource.ResourceServiceClient, linkEventHandler LinkEventHandler,
) {
	errorBackoff := &retry.Waiter{
		MinFailures: 10,
		MinWait:     0,
		MaxWait:     1 * time.Minute,
	}
	for {
		select {
		case <-ctx.Done():
			logger.Debug("context canceled, exiting")
			return
		default:
			watchClient, err := client.WatchList(
				ctx, &pbresource.WatchListRequest{
					Type:       pbhcp.LinkType,
					NamePrefix: hcpctl.LinkName,
				},
			)
			if err != nil {
				logger.Error("failed to create watch on Link", "error", err)
				errorBackoff.Wait(ctx)
				continue
			}
			errorBackoff.Reset()
			handleLinkEvents(ctx, logger, watchClient, linkEventHandler)
		}
	}
}
