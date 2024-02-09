// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"

	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	hcpctl "github.com/hashicorp/consul/internal/hcp"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// This tests that when we get a watch event from the Recv call, we get that same event on the
// output channel, then we
func TestLinkWatcher_Ok(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	testWatchEvent := &pbresource.WatchEvent{}
	mockWatchListClient := mockpbresource.NewResourceService_WatchListClient(t)
	mockWatchListClient.EXPECT().Recv().Return(testWatchEvent, nil)

	eventCh := make(chan *pbresource.WatchEvent)
	mockLinkHandler := func(_ context.Context, _ hclog.Logger, event *pbresource.WatchEvent) {
		eventCh <- event
	}

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: hcpctl.LinkName,
	}).Return(mockWatchListClient, nil)

	go RunHCPLinkWatcher(ctx, hclog.Default(), client, mockLinkHandler)

	// Assert that the link handler is called with the testWatchEvent
	receivedWatchEvent := <-eventCh
	require.Equal(t, testWatchEvent, receivedWatchEvent)
}

func TestLinkWatcher_RecvError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Our mock WatchListClient will simulate 5 errors, then will cancel the context.
	// We expect RunHCPLinkWatcher to attempt to create the WatchListClient 6 times (initial attempt plus 5 retries)
	// before exiting due to context cancellation.
	mockWatchListClient := mockpbresource.NewResourceService_WatchListClient(t)
	numFailures := 5
	failures := 0
	mockWatchListClient.EXPECT().Recv().RunAndReturn(func() (*pbresource.WatchEvent, error) {
		if failures < numFailures {
			failures++
			return nil, errors.New("unexpectedError")
		}
		defer cancel()
		return &pbresource.WatchEvent{}, nil
	})

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: hcpctl.LinkName,
	}).Return(mockWatchListClient, nil).Times(numFailures + 1)

	RunHCPLinkWatcher(ctx, hclog.Default(), client, func(_ context.Context, _ hclog.Logger, _ *pbresource.WatchEvent) {})
}

func TestLinkWatcher_WatchListError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Our mock WatchList will simulate 5 errors, then will cancel the context.
	// We expect RunHCPLinkWatcher to attempt to create the WatchListClient 6 times (initial attempt plus 5 retries)
	// before exiting due to context cancellation.
	numFailures := 5
	failures := 0

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: hcpctl.LinkName,
	}).RunAndReturn(func(_ context.Context, _ *pbresource.WatchListRequest, _ ...grpc.CallOption) (pbresource.ResourceService_WatchListClient, error) {
		if failures < numFailures {
			failures++
			return nil, errors.New("unexpectedError")
		}
		defer cancel()
		return mockpbresource.NewResourceService_WatchListClient(t), nil
	}).Times(numFailures + 1)

	RunHCPLinkWatcher(ctx, hclog.Default(), client, func(_ context.Context, _ hclog.Logger, _ *pbresource.WatchEvent) {})
}
