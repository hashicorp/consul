// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestHandleStream_DuplicateConnectionDoesNotCorruptStatus tests that when a second
// stream tries to connect for the same peer ID while an active stream exists,
// the active stream's status is NOT corrupted.
//
// This is a regression test for INFRA-14028 where HandleStream unconditionally
// called DisconnectedDueToError even when Connected() failed, which corrupted
// the active stream's status causing consul_peering_healthy to incorrectly
// report unhealthy peerings.
func TestHandleStream_DuplicateConnectionDoesNotCorruptStatus(t *testing.T) {
	// Disable ConnectEnabled to skip trust domain checks which would require CA setup
	srv, _ := newTestServer(t, func(c *Config) {
		c.ConnectEnabled = false
	})
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	// Create context for Stream A that we can cancel to clean up
	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()

	// Create Stream A
	streamA := newTestReplicationStream(ctxA)

	// Track when Stream A has connected
	streamAConnected := make(chan struct{})
	streamADone := make(chan error, 1)

	// Start Stream A in a goroutine - it will connect and then wait for messages
	go func() {
		// Signal when we've started (Connected() should be called very quickly)
		go func() {
			// Give a small window for Connected() to be called
			time.Sleep(50 * time.Millisecond)
			close(streamAConnected)
		}()

		err := srv.HandleStream(HandleStreamRequest{
			LocalID:  peerID,
			RemoteID: "remote-peer-id",
			PeerName: "my-peer",
			Stream:   streamA,
		})
		streamADone <- err
	}()

	// Wait for Stream A to connect
	<-streamAConnected

	// Verify Stream A is connected
	status, ok := srv.Tracker.StreamStatus(peerID)
	require.True(t, ok, "Stream A should have registered with tracker")
	require.True(t, status.Connected, "Stream A should be connected")
	require.Nil(t, status.DisconnectTime, "Stream A should have no DisconnectTime")

	// Create context for Stream B
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	// Create Stream B with the same peer ID
	streamB := newTestReplicationStream(ctxB)

	// Stream B tries to connect - this should fail because Stream A is active
	var wg sync.WaitGroup
	wg.Add(1)
	var streamBErr error
	go func() {
		defer wg.Done()
		streamBErr = srv.HandleStream(HandleStreamRequest{
			LocalID:  peerID, // Same peer ID as Stream A
			RemoteID: "remote-peer-id-2",
			PeerName: "my-peer",
			Stream:   streamB,
		})
	}()

	// Wait for Stream B to finish (it should fail quickly at Connected())
	wg.Wait()

	// Stream B should have failed
	require.Error(t, streamBErr, "Stream B should have failed to connect")
	require.Contains(t, streamBErr.Error(), "there is an active stream",
		"Stream B should fail because Stream A is already active")

	// CRITICAL: Verify Stream A's status was NOT corrupted by Stream B's failed attempt
	status, ok = srv.Tracker.StreamStatus(peerID)
	require.True(t, ok, "Stream A should still be in tracker")
	require.True(t, status.Connected,
		"BUG: Stream A's Connected status was corrupted by Stream B's failed connection attempt")
	require.Nil(t, status.DisconnectTime,
		"BUG: Stream A's DisconnectTime was set by Stream B even though Stream A is still active")

	// Clean up: close Stream A's receive channel to simulate client disconnect
	// This causes Recv() to return io.EOF which terminates the stream handler
	close(streamA.recvCh)
	cancelA()

	// Wait for Stream A to finish
	select {
	case <-streamADone:
		// Stream A finished
	case <-time.After(5 * time.Second):
		t.Fatal("Stream A did not finish after context cancellation")
	}
}

// TestHandleStream_NormalDisconnectUpdatesStatus verifies that when a stream
// disconnects normally (not due to a duplicate connection), the status IS updated.
// This ensures the fix doesn't break normal disconnect handling.
func TestHandleStream_NormalDisconnectUpdatesStatus(t *testing.T) {
	// Disable ConnectEnabled to skip trust domain checks which would require CA setup
	srv, _ := newTestServer(t, func(c *Config) {
		c.ConnectEnabled = false
	})
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	// Create context that we'll cancel to simulate disconnect
	ctx, cancel := context.WithCancel(context.Background())

	stream := newTestReplicationStream(ctx)

	streamDone := make(chan error, 1)

	go func() {
		err := srv.HandleStream(HandleStreamRequest{
			LocalID:  peerID,
			RemoteID: "remote-peer-id",
			PeerName: "my-peer",
			Stream:   stream,
		})
		streamDone <- err
	}()

	// Wait for stream to connect
	require.Eventually(t, func() bool {
		status, ok := srv.Tracker.StreamStatus(peerID)
		return ok && status.Connected
	}, 5*time.Second, 10*time.Millisecond, "Stream should connect")

	// Close the receive channel to simulate client disconnect
	// This causes Recv() to return io.EOF which terminates the stream handler
	close(stream.recvCh)
	cancel()

	// Wait for HandleStream to return
	select {
	case <-streamDone:
		// Stream finished
	case <-time.After(5 * time.Second):
		t.Fatal("Stream did not finish after context cancellation")
	}

	// After a normal disconnect, the status SHOULD be updated to disconnected
	status, ok := srv.Tracker.StreamStatus(peerID)
	require.True(t, ok, "Peer should still be in tracker")
	require.False(t, status.Connected, "Stream should be marked as disconnected after normal disconnect")
}

// TestTracker_ConnectedFailsForDuplicatePeerID is a unit test that verifies
// the Tracker.Connected() method correctly rejects duplicate peer IDs.
func TestTracker_ConnectedFailsForDuplicatePeerID(t *testing.T) {
	tracker := NewTracker(defaultIncomingHeartbeatTimeout)
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	// First connection succeeds
	status, _, err := tracker.Connected(peerID)
	require.NoError(t, err)
	require.True(t, status.Connected)

	// Second connection with same peer ID fails
	_, _, err = tracker.Connected(peerID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "there is an active stream for the given PeerID")

	// Original status should be unchanged
	streamStatus, ok := tracker.StreamStatus(peerID)
	require.True(t, ok)
	require.True(t, streamStatus.Connected)
	require.Nil(t, streamStatus.DisconnectTime)
}

// TestHandleStream_StaleDisconnectDoesNotCorruptNewStream tests that when a stale
// stream goroutine's disconnect fires after a new stream has connected, the new
// stream's status is not corrupted.
func TestHandleStream_StaleDisconnectDoesNotCorruptNewStream(t *testing.T) {
	srv, _ := newTestServer(t, func(c *Config) {
		c.ConnectEnabled = false
	})
	peerID := "63b60245-c475-426b-b314-4588d210859d"

	// Simulate: Stream A connects (gen=1), then disconnects
	ctxA, cancelA := context.WithCancel(context.Background())
	streamA := newTestReplicationStream(ctxA)

	streamADone := make(chan error, 1)
	go func() {
		streamADone <- srv.HandleStream(HandleStreamRequest{
			LocalID:  peerID,
			RemoteID: "remote-a",
			PeerName: "my-peer",
			Stream:   streamA,
		})
	}()

	// Wait for Stream A to connect
	require.Eventually(t, func() bool {
		status, ok := srv.Tracker.StreamStatus(peerID)
		return ok && status.Connected
	}, 5*time.Second, 10*time.Millisecond)

	// Disconnect Stream A
	close(streamA.recvCh)
	cancelA()
	select {
	case <-streamADone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stream A did not finish")
	}

	// Verify Stream A is disconnected
	status, ok := srv.Tracker.StreamStatus(peerID)
	require.True(t, ok)
	require.False(t, status.Connected)

	// Now Stream B connects (gen=2)
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()
	streamB := newTestReplicationStream(ctxB)

	streamBDone := make(chan error, 1)
	go func() {
		streamBDone <- srv.HandleStream(HandleStreamRequest{
			LocalID:  peerID,
			RemoteID: "remote-b",
			PeerName: "my-peer",
			Stream:   streamB,
		})
	}()

	// Wait for Stream B to connect
	require.Eventually(t, func() bool {
		status, ok := srv.Tracker.StreamStatus(peerID)
		return ok && status.Connected
	}, 5*time.Second, 10*time.Millisecond)

	// Simulate stale disconnect from Stream A's generation (gen=1)
	// This should be a no-op because Stream B has gen=2
	srv.Tracker.DisconnectedDueToError(peerID, 1, "stale error from old stream")

	// Verify Stream B is still connected
	status, ok = srv.Tracker.StreamStatus(peerID)
	require.True(t, ok)
	require.True(t, status.Connected,
		"BUG: stale disconnect from old stream corrupted new stream's status")
	require.Nil(t, status.DisconnectTime,
		"BUG: stale disconnect set DisconnectTime on new stream")

	// Clean up
	close(streamB.recvCh)
	cancelB()
	select {
	case <-streamBDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Stream B did not finish")
	}
}
