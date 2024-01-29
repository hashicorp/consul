// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/hcp/internal/types"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var linkWatchRetryTime = time.Second

func NewLinkWatch(ctx context.Context, logger hclog.Logger, client pbresource.ResourceServiceClient) (chan *pbresource.WatchEvent, error) {
	watchClient, err := client.WatchList(
		ctx, &pbresource.WatchListRequest{
			Type:       pbhcp.LinkType,
			NamePrefix: types.LinkName,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create watch on Link: %w", err)
	}

	eventCh := make(chan *pbresource.WatchEvent)
	go func() {
		for {
			watchEvent, err := watchClient.Recv()
			if err != nil {
				select {
				case <-ctx.Done():
					logger.Debug("context canceled, exiting")
					close(eventCh)
					return
				default:
					logger.Error("error receiving link watch event", "error", err)

					// In case of an error, wait before retrying, so we don't log errors in a fast loop
					time.Sleep(linkWatchRetryTime)
					continue
				}

			}
			eventCh <- watchEvent
		}
	}()

	return eventCh, nil
}
