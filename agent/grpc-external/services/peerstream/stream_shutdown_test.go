// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
)

// shutdownTestStream is a BidirectionalStream whose Send can be made to fail for selected
// messages and whose Recv returns queued messages, then io.EOF (or recvErr) once in is closed.
type shutdownTestStream struct {
	ctx      context.Context
	in       chan *pbpeerstream.ReplicationMessage
	recvErr  error
	failSend func(*pbpeerstream.ReplicationMessage) error

	mu   sync.Mutex
	sent []*pbpeerstream.ReplicationMessage
}

func newShutdownTestStream(t *testing.T) *shutdownTestStream {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &shutdownTestStream{ctx: ctx, in: make(chan *pbpeerstream.ReplicationMessage, 16)}
}

func (s *shutdownTestStream) Context() context.Context { return s.ctx }

func (s *shutdownTestStream) Send(msg *pbpeerstream.ReplicationMessage) error {
	if s.failSend != nil {
		if err := s.failSend(msg); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, msg)
	return nil
}

func (s *shutdownTestStream) Recv() (*pbpeerstream.ReplicationMessage, error) {
	select {
	case msg, ok := <-s.in:
		if !ok {
			if s.recvErr != nil {
				return nil, s.recvErr
			}
			return nil, io.EOF
		}
		return msg, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *shutdownTestStream) sentNACK(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, msg := range s.sent {
		if r := msg.GetRequest(); r != nil && r.ResponseNonce == nonce && r.Error != nil {
			return true
		}
	}
	return false
}

// badResponse is a response for an unknown resource type, which the handler answers with a NACK.
func badResponse(nonce string) *pbpeerstream.ReplicationMessage {
	return &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Response_{
			Response: &pbpeerstream.ReplicationMessage_Response{ResourceURL: "unknown-type", Nonce: nonce},
		},
	}
}

type terminateRecordingBackend struct {
	*testStreamBackend
	terminated chan string
}

func (b *terminateRecordingBackend) PeeringTerminateByID(req *pbpeering.PeeringTerminateByIDRequest) error {
	b.terminated <- req.ID
	return nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func setupShutdownTest(t *testing.T, configFn func(c *Config)) (*testServer, *pbpeering.Peering) {
	t.Helper()
	srv, store := newTestServer(t, configFn)
	p := writePeeringToBeDialed(t, store, 1, "my-peer")
	_, _ = writeInitialRootsAndCA(t, store)
	return srv, p
}

func startHandleStream(t *testing.T, srv *testServer, req HandleStreamRequest) <-chan error {
	t.Helper()
	errCh := make(chan error, 1)
	go func() { errCh <- srv.HandleStream(req) }()
	require.Eventually(t, func() bool {
		status, ok := srv.StreamStatus(req.LocalID)
		return ok && status.Connected
	}, 10*time.Second, 10*time.Millisecond, "stream never connected")
	return errCh
}

func waitHandleStream(t *testing.T, errCh <-chan error) error {
	t.Helper()
	select {
	case err := <-errCh:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("HandleStream did not return")
		return nil
	}
}

func TestHandleStream_PeerTerminated(t *testing.T) {
	var backend *terminateRecordingBackend
	srv, p := setupShutdownTest(t, func(c *Config) {
		backend = &terminateRecordingBackend{
			testStreamBackend: c.Backend.(*testStreamBackend),
			terminated:        make(chan string, 1),
		}
		c.Backend = backend
	})

	stream := newShutdownTestStream(t)
	req := HandleStreamRequest{LocalID: p.ID, PeerName: p.Name, Stream: stream}
	errCh := startHandleStream(t, srv, req)

	stream.in <- &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Terminated_{Terminated: &pbpeerstream.ReplicationMessage_Terminated{}},
	}
	require.NoError(t, waitHandleStream(t, errCh))

	select {
	case id := <-backend.terminated:
		require.Equal(t, p.ID, id)
	case <-time.After(10 * time.Second):
		t.Fatal("peering was not marked as terminated")
	}

	status, ok := srv.StreamStatus(p.ID)
	require.True(t, ok)
	require.False(t, status.Connected)
	require.Empty(t, status.DisconnectErrorMessage)

	// After the handler returns, its receiver goroutine keeps reading and discarding messages,
	// and DrainStream waits for it until the peer closes the stream.
	stream.in <- &pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Heartbeat_{Heartbeat: &pbpeerstream.ReplicationMessage_Heartbeat{}},
	}
	drained := make(chan struct{})
	go func() {
		srv.DrainStream(req)
		close(drained)
	}()
	select {
	case <-drained:
		t.Fatal("DrainStream returned before the peer closed the stream")
	case <-time.After(100 * time.Millisecond):
	}
	close(stream.in)
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
		t.Fatal("DrainStream did not return after the peer closed the stream")
	}

	_, found := srv.receivers.Load(req.Stream)
	require.False(t, found, "receiver should be removed after DrainStream")
}

func TestHandleStream_AckSendErrorEndsStream(t *testing.T) {
	srv, p := setupShutdownTest(t, nil)

	stream := newShutdownTestStream(t)
	stream.failSend = func(msg *pbpeerstream.ReplicationMessage) error {
		if r := msg.GetRequest(); r != nil && r.ResponseNonce != "" {
			return errors.New("boom")
		}
		return nil
	}
	req := HandleStreamRequest{LocalID: p.ID, PeerName: p.Name, Stream: stream}
	errCh := startHandleStream(t, srv, req)

	// The NACK for this response is written by the sender goroutine; its failure must end the stream.
	stream.in <- badResponse("1")
	err := waitHandleStream(t, errCh)
	require.ErrorContains(t, err, "failed to send to stream: boom")

	status, ok := srv.StreamStatus(p.ID)
	require.True(t, ok)
	require.False(t, status.Connected)
	require.Contains(t, status.DisconnectErrorMessage, "boom")

	_, found := srv.receivers.Load(req.Stream)
	require.False(t, found, "receiver should be removed when the stream ends with an error")
}

func TestHandleStream_HeartbeatSendErrorIsNotFatal(t *testing.T) {
	srv, p := setupShutdownTest(t, func(c *Config) {
		c.outgoingHeartbeatInterval = 10 * time.Millisecond
	})

	var heartbeatFailures atomic.Int32
	stream := newShutdownTestStream(t)
	stream.failSend = func(msg *pbpeerstream.ReplicationMessage) error {
		if msg.GetHeartbeat() != nil {
			heartbeatFailures.Add(1)
			return errors.New("boom")
		}
		return nil
	}
	req := HandleStreamRequest{LocalID: p.ID, PeerName: p.Name, Stream: stream}
	errCh := startHandleStream(t, srv, req)

	require.Eventually(t, func() bool { return heartbeatFailures.Load() >= 3 },
		10*time.Second, 10*time.Millisecond, "heartbeats were not retried after a send error")

	// The stream is still fully functional: a bad response is still answered with a NACK.
	stream.in <- badResponse("7")
	require.Eventually(t, func() bool { return stream.sentNACK("7") },
		10*time.Second, 10*time.Millisecond, "NACK was not sent")

	select {
	case err := <-errCh:
		t.Fatalf("stream ended after a heartbeat send error: %v", err)
	default:
	}

	close(srv.ConnectedStreams()[p.ID])
	require.NoError(t, waitHandleStream(t, errCh))
}

func TestHandleStream_TerminateSendErrorReturnsError(t *testing.T) {
	srv, p := setupShutdownTest(t, nil)

	stream := newShutdownTestStream(t)
	stream.failSend = func(msg *pbpeerstream.ReplicationMessage) error {
		if msg.GetTerminated() != nil {
			return errors.New("boom")
		}
		return nil
	}
	req := HandleStreamRequest{LocalID: p.ID, PeerName: p.Name, Stream: stream}
	errCh := startHandleStream(t, srv, req)

	// Delete the peering locally, the same way the leader does.
	close(srv.ConnectedStreams()[p.ID])
	err := waitHandleStream(t, errCh)
	require.ErrorContains(t, err, "failed to send to stream: boom")

	_, found := srv.receivers.Load(req.Stream)
	require.False(t, found, "receiver should be removed when the stream ends with an error")
}

func TestServer_DrainStream_WithoutReceiver(t *testing.T) {
	logs := &lockedBuffer{}
	srv, _ := setupShutdownTest(t, func(c *Config) {
		c.Logger = hclog.New(&hclog.LoggerOptions{Output: logs, Level: hclog.Warn})
	})

	t.Run("peer closes the stream", func(t *testing.T) {
		stream := newShutdownTestStream(t)
		stream.in <- badResponse("1")
		close(stream.in)

		srv.DrainStream(HandleStreamRequest{LocalID: "id", PeerName: "peer", Stream: stream})
		require.NotContains(t, logs.String(), "failed to tear down stream gracefully")
	})

	t.Run("stream fails", func(t *testing.T) {
		stream := newShutdownTestStream(t)
		stream.recvErr = errors.New("connection reset")
		stream.in <- badResponse("1")
		close(stream.in)

		srv.DrainStream(HandleStreamRequest{LocalID: "id", PeerName: "peer", Stream: stream})
		require.Contains(t, logs.String(), "failed to tear down stream gracefully")
		require.Contains(t, logs.String(), "connection reset")
	})
}
