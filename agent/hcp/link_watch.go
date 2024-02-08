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

func StartLinkWatch(
	ctx context.Context, logger hclog.Logger, client pbresource.ResourceServiceClient,
) chan *pbresource.WatchEvent {
	var watchClient pbresource.ResourceService_WatchListClient
	watchListErrorBackoff := &retry.Waiter{
		MinFailures: 1,
		MinWait:     1 * time.Second,
		MaxWait:     1 * time.Minute,
	}
	for {
		var err error
		watchClient, err = client.WatchList(
			ctx, &pbresource.WatchListRequest{
				Type:       pbhcp.LinkType,
				NamePrefix: hcpctl.LinkName,
			},
		)
		if err != nil {
			logger.Error("failed to create watch on Link", "error", err)
			watchListErrorBackoff.Wait(ctx)
			continue
		}

		break
	}

	eventCh := make(chan *pbresource.WatchEvent)
	go func() {
		errorBackoff := &retry.Waiter{
			MinFailures: 1,
			MinWait:     1 * time.Second,
			MaxWait:     1 * time.Minute,
		}
		for {
			select {
			case <-ctx.Done():
				logger.Debug("context canceled, exiting")
				close(eventCh)
				return
			default:
				watchEvent, err := watchClient.Recv()

				if err != nil {
					logger.Error("error receiving link watch event", "error", err)
					errorBackoff.Wait(ctx)
					continue
				}

				errorBackoff.Reset()
				eventCh <- watchEvent
			}
		}
	}()

	return eventCh
}
