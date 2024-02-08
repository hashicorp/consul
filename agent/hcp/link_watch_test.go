// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package hcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	mockpbresource "github.com/hashicorp/consul/grpcmocks/proto-public/pbresource"
	hcpctl "github.com/hashicorp/consul/internal/hcp"
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
		NamePrefix: hcpctl.LinkName,
	}).Return(mockWatchListClient, nil)
	linkWatchCh := StartLinkWatch(context.Background(), hclog.Default(), client)

	event := <-linkWatchCh
	require.Equal(t, testWatchEvent, event)
}

// This tests ensures that when the context is canceled, the linkWatchCh is closed
func TestLinkWatch_ContextCanceled(t *testing.T) {
	mockWatchListClient := mockpbresource.NewResourceService_WatchListClient(t)
	// Recv may not be called if the context is cancelled before the `select` block
	// within the StartLinkWatch goroutine
	mockWatchListClient.EXPECT().Recv().Return(nil, errors.New("context canceled")).Maybe()

	client := mockpbresource.NewResourceServiceClient(t)
	client.EXPECT().WatchList(mock.Anything, &pbresource.WatchListRequest{
		Type:       pbhcp.LinkType,
		NamePrefix: hcpctl.LinkName,
	}).Return(mockWatchListClient, nil)

	ctx, cancel := context.WithCancel(context.Background())
	linkWatchCh := StartLinkWatch(ctx, hclog.Default(), client)

	cancel()

	// Ensure the linkWatchCh is closed
	_, ok := <-linkWatchCh
	require.False(t, ok)
}
