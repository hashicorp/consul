// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	"github.com/hashicorp/consul/internal/hcp/internal/types"
	pbhcp "github.com/hashicorp/consul/proto-public/pbhcp/v2"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// This tests that when we get a watch event from the Recv call, we get that same event on the
// output channel, then we
func TestLinkWatch_Ok(t *testing.T) {
	testWatchEvent := &pbresource.WatchEvent{}

	mockWatchListClient := mockpbresource.NewResourceService_WatchListClient(t)
	mockWatchListClient.EXPECT().Recv().Return(testWatchEvent, nil)

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: types.LinkName,
	}).Return(mockWatchListClient, nil)
	linkWatchCh, err := NewLinkWatch(context.Background(), hclog.Default(), client)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		select {
		case event := <-linkWatchCh:
			return event == testWatchEvent
		default:
			return false
		}
	}, 10*time.Millisecond, time.Millisecond)
}

// This tests ensures that when the context is canceled, the linkWatchCh is closed
func TestLinkWatch_ContextCanceled(t *testing.T) {
	mockWatchListClient := mockpbresource.NewResourceService_WatchListClient(t)
	// Recv may not be called if the context is cancelled before the `select` block
	// within the NewLinkWatch goroutine
	mockWatchListClient.EXPECT().Recv().Return(nil, errors.New("context canceled")).Maybe()

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: types.LinkName,
	}).Return(mockWatchListClient, nil)

	ctx, cancel := context.WithCancel(context.Background())
	linkWatchCh, err := NewLinkWatch(ctx, hclog.Default(), client)
	require.NoError(t, err)

	cancel()

	// Ensure the linkWatchCh is closed
	_, ok := <-linkWatchCh
	require.False(t, ok)
}

// This tests ensures that when Recv returns errors repeatedly, we eventually close the channel
// and exit the goroutine
func TestLinkWatch_RepeatErrors(t *testing.T) {
	mockWatchListClient := mockpbresource.NewResourceService_WatchListClient(t)
	// Recv should be called 10 times and no more since it is repeatedly returning an error.
	mockWatchListClient.EXPECT().Recv().Return(nil, errors.New("unexpected error")).Times(linkWatchRetryCount)

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: types.LinkName,
	}).Return(mockWatchListClient, nil)

	linkWatchCh, err := NewLinkWatch(context.Background(), hclog.Default(), client)
	require.NoError(t, err)

	// Ensure the linkWatchCh is closed
	_, ok := <-linkWatchCh
	require.False(t, ok)
}
