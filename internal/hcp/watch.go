// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/internal/hcp/internal/types"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

var linkWatchRetryCount = 10

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
		errorCounter := 0
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

					errorCounter++
					if errorCounter >= linkWatchRetryCount {
						logger.Error("received multiple consecutive errors from link watch client, will stop watching link")
						close(eventCh)
						return
					}

					continue
				}
				errorCounter = 0
				eventCh <- watchEvent
			}
		}
	}()

	return eventCh, nil
}
