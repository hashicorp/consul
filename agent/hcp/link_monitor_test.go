// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestMonitorHCPLink_Ok(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	linkWatchCh := make(chan *pbresource.WatchEvent)
	mgr := NewMockManager(t)
	mgr.EXPECT().Start(mock.Anything).Return(nil).Once()
	mgr.EXPECT().Stop().Return(nil).Once()

	go MonitorHCPLink(ctx, hclog.New(&hclog.LoggerOptions{Output: io.Discard}), linkWatchCh, mgr)

	linkWatchCh <- &pbresource.WatchEvent{Operation: pbresource.WatchEvent_OPERATION_UPSERT}
	linkWatchCh <- &pbresource.WatchEvent{Operation: pbresource.WatchEvent_OPERATION_DELETE}

	cancel()
}
