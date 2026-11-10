// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/stream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
	"github.com/hashicorp/consul/proto/private/pbstatus"
	"github.com/hashicorp/consul/sdk/testutil"
)

// TestHandleStream_BidirectionalFlowControl connects an acceptor and a dialer
// over a real gRPC transport, with both sides exporting many services. Both
// sides push their full snapshot concurrently, which is what happens after a
// server restart. The stream must not stall on HTTP/2 flow control.
func TestHandleStream_BidirectionalFlowControl(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	const (
		numServices  = 715
		metaValueLen = 2048
	)

	newSide := func(t *testing.T) (*Server, *state.Store) {
		publisher := stream.NewEventPublisher(10 * time.Second)
		store, _ := newStateStore(t, publisher)
		srv := NewServer(Config{
			Backend:        &flowControlTestBackend{testStreamBackend: &testStreamBackend{store: store, pub: publisher}},
			GetStore:       func() StateStore { return store },
			Logger:         hclog.New(&hclog.LoggerOptions{Level: hclog.Warn, Output: testutil.NewLogBuffer(t)}),
			Datacenter:     "dc1",
			ConnectEnabled: true,
			ForwardRPC:     noopForwardRPC,

			outgoingHeartbeatInterval: time.Second,
		})
		return srv, store
	}

	populate := func(t *testing.T, store *state.Store, prefix, consumerPeer string) {
		idx := uint64(100)
		require.NoError(t, store.EnsureConfigEntry(idx, &structs.ExportedServicesConfigEntry{
			Name: "default",
			Services: []structs.ExportedService{{
				Name:      structs.WildcardSpecifier,
				Consumers: []structs.ServiceConsumer{{Peer: consumerPeer}},
			}},
		}))
		idx++
		node := &structs.Node{Node: prefix + "-node", Address: "10.0.0.1"}
		require.NoError(t, store.EnsureNode(idx, node))
		meta := map[string]string{}
		for k := 0; k < metaValueLen/512; k++ {
			meta[fmt.Sprintf("pad%d", k)] = strings.Repeat("x", 500)
		}
		for i := 0; i < numServices; i++ {
			idx++
			name := fmt.Sprintf("%s-%04d", prefix, i)
			require.NoError(t, store.EnsureService(idx, node.Node, &structs.NodeService{
				ID:      name,
				Service: name,
				Port:    8080,
				Meta:    meta,
			}))
		}
	}

	// Acceptor side.
	acceptor, acceptorStore := newSide(t)
	p := writePeeringToBeDialed(t, acceptorStore, 1, "dialer-peer")
	_, _ = writeInitialRootsAndCA(t, acceptorStore)
	populate(t, acceptorStore, "acc", "dialer-peer")

	grpcServer := grpc.NewServer()
	acceptor.Register(grpcServer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = grpcServer.Serve(ln) }()
	t.Cleanup(grpcServer.Stop)

	// Dialer side.
	dialer, dialerStore := newSide(t)
	require.NoError(t, dialerStore.PeeringWrite(1, &pbpeering.PeeringWriteRequest{
		Peering: &pbpeering.Peering{
			ID:                  testPeerID,
			Name:                "acceptor-peer",
			PeerID:              p.ID,
			PeerServerAddresses: []string{ln.Addr().String()},
		},
	}))
	_, dp, err := dialerStore.PeeringRead(nil, state.Query{Value: "acceptor-peer"})
	require.NoError(t, err)
	_, _ = writeInitialRootsAndCA(t, dialerStore)
	populate(t, dialerStore, "dial", "acceptor-peer")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	clientStream, err := pbpeerstream.NewPeerStreamServiceClient(conn).StreamResources(ctx)
	require.NoError(t, err)
	require.NoError(t, clientStream.Send(&pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Open_{
			Open: &pbpeerstream.ReplicationMessage_Open{
				PeerID:         dp.PeerID,
				StreamSecretID: testPendingStreamSecretID,
			},
		},
	}))

	go func() {
		_ = dialer.HandleStream(HandleStreamRequest{
			LocalID:  dp.ID,
			RemoteID: dp.PeerID,
			PeerName: dp.Name,
			Stream:   clientStream,
		})
	}()

	// Each side exports numServices services; the imported list must converge on both sides.
	deadline := time.Now().Add(60 * time.Second)
	var accStatus, dialStatus Status
	for time.Now().Before(deadline) {
		accStatus, _ = acceptor.StreamStatus(p.ID)
		dialStatus, _ = dialer.StreamStatus(dp.ID)
		if accStatus.GetImportedServicesCount() == numServices &&
			dialStatus.GetImportedServicesCount() == numServices &&
			accStatus.LastRecvHeartbeat != nil && dialStatus.LastRecvHeartbeat != nil {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}

	var sb strings.Builder
	_ = pprof.Lookup("goroutine").WriteTo(&sb, 2)
	var stuck []string
	for _, g := range strings.Split(sb.String(), "\n\n") {
		if strings.Contains(g, "peerstream.(*Server).realHandleStream") {
			stuck = append(stuck, g)
		}
	}
	t.Fatalf("peering stream did not converge: acceptor imported=%d exported=%d hb=%v lastSend=%v lastRecv=%v; "+
		"dialer imported=%d exported=%d hb=%v lastSend=%v lastRecv=%v\n\nrealHandleStream goroutines:\n%s",
		accStatus.GetImportedServicesCount(), accStatus.GetExportedServicesCount(), accStatus.LastRecvHeartbeat, accStatus.LastSendSuccess, accStatus.LastRecvResourceSuccess,
		dialStatus.GetImportedServicesCount(), dialStatus.GetExportedServicesCount(), dialStatus.LastRecvHeartbeat, dialStatus.LastSendSuccess, dialStatus.LastRecvResourceSuccess,
		strings.Join(stuck, "\n\n"))
}

type flowControlTestBackend struct {
	*testStreamBackend
}

func (b *flowControlTestBackend) PeeringTrustBundleWrite(req *pbpeering.PeeringTrustBundleWriteRequest) error {
	return b.store.PeeringTrustBundleWrite(1, req.PeeringTrustBundle)
}

// TestHandleStream_LocalDeletionWhileSendBlocked deletes a peering locally while
// the stream's sender is blocked on HTTP/2 flow control. The peer behaves like
// an unpatched server: it does not read until it has finished writing a large
// burst of messages. The terminating side must keep draining received messages
// while its in-flight write and the Terminated message are pending, otherwise
// both sides wait for each other forever.
func TestHandleStream_LocalDeletionWhileSendBlocked(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	const (
		numServices = 715
		numNacks    = 200
		nackSize    = 32 * 1024
	)

	publisher := stream.NewEventPublisher(10 * time.Second)
	store, _ := newStateStore(t, publisher)
	acceptor := NewServer(Config{
		Backend:        &flowControlTestBackend{testStreamBackend: &testStreamBackend{store: store, pub: publisher}},
		GetStore:       func() StateStore { return store },
		Logger:         hclog.New(&hclog.LoggerOptions{Level: hclog.Error, Output: testutil.NewLogBuffer(t)}),
		Datacenter:     "dc1",
		ConnectEnabled: true,
		ForwardRPC:     noopForwardRPC,
	})

	p := writePeeringToBeDialed(t, store, 1, "dialer-peer")
	_, _ = writeInitialRootsAndCA(t, store)
	idx := uint64(100)
	require.NoError(t, store.EnsureConfigEntry(idx, &structs.ExportedServicesConfigEntry{
		Name: "default",
		Services: []structs.ExportedService{{
			Name:      structs.WildcardSpecifier,
			Consumers: []structs.ServiceConsumer{{Peer: "dialer-peer"}},
		}},
	}))
	idx++
	node := &structs.Node{Node: "acc-node", Address: "10.0.0.1"}
	require.NoError(t, store.EnsureNode(idx, node))
	meta := map[string]string{}
	for k := 0; k < 4; k++ {
		meta[fmt.Sprintf("pad%d", k)] = strings.Repeat("x", 500)
	}
	for i := 0; i < numServices; i++ {
		idx++
		name := fmt.Sprintf("acc-%04d", i)
		require.NoError(t, store.EnsureService(idx, node.Node, &structs.NodeService{ID: name, Service: name, Port: 8080, Meta: meta}))
	}

	grpcServer := grpc.NewServer()
	acceptor.Register(grpcServer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = grpcServer.Serve(ln) }()
	t.Cleanup(grpcServer.Stop)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	conn, err := grpc.NewClient(ln.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	client, err := pbpeerstream.NewPeerStreamServiceClient(conn).StreamResources(ctx)
	require.NoError(t, err)
	require.NoError(t, client.Send(&pbpeerstream.ReplicationMessage{
		Payload: &pbpeerstream.ReplicationMessage_Open_{
			Open: &pbpeerstream.ReplicationMessage_Open{PeerID: p.ID, StreamSecretID: testPendingStreamSecretID},
		},
	}))
	for _, url := range []string{
		pbpeerstream.TypeURLExportedService,
		pbpeerstream.TypeURLExportedServiceList,
		pbpeerstream.TypeURLPeeringTrustBundle,
		pbpeerstream.TypeURLPeeringServerAddresses,
	} {
		require.NoError(t, client.Send(makeReplicationRequest(&pbpeerstream.ReplicationMessage_Request{
			PeerID:      p.ID,
			ResourceURL: url,
		})))
	}

	// The client does not read, so the acceptor's snapshot fills the flow-control
	// window and its sender blocks in Stream.Send.
	require.Eventually(t, func() bool {
		var sb strings.Builder
		_ = pprof.Lookup("goroutine").WriteTo(&sb, 2)
		for _, g := range strings.Split(sb.String(), "\n\n") {
			if strings.Contains(g, "writeQuota") && strings.Contains(g, "peerstream.(*Server).realHandleStream") {
				return true
			}
		}
		return false
	}, 30*time.Second, 100*time.Millisecond, "acceptor sender never blocked on flow control")

	// Delete the peering locally, the same way the leader does.
	doneCh, ok := acceptor.ConnectedStreams()[p.ID]
	require.True(t, ok)
	close(doneCh)

	// Behave like an unpatched peer: write a large burst first and only start
	// reading once every write has completed.
	gotTerminated := make(chan error, 1)
	go func() {
		bigNack := strings.Repeat("e", nackSize)
		for i := 0; i < numNacks; i++ {
			err := client.Send(makeReplicationRequest(&pbpeerstream.ReplicationMessage_Request{
				ResourceURL:   pbpeerstream.TypeURLExportedService,
				ResponseNonce: "1",
				Error:         &pbstatus.Status{Code: 13, Message: bigNack},
			}))
			if err != nil {
				gotTerminated <- fmt.Errorf("send %d: %w", i, err)
				return
			}
		}
		for {
			msg, err := client.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = errors.New("stream closed without a Terminated message")
				}
				gotTerminated <- err
				return
			}
			if msg.GetTerminated() != nil {
				// Like a real peer, close our side after receiving Terminated and read until
				// the acceptor finishes the stream.
				if err := client.CloseSend(); err != nil {
					gotTerminated <- err
					return
				}
				for {
					if _, err := client.Recv(); err != nil {
						if errors.Is(err, io.EOF) {
							err = nil
						}
						gotTerminated <- err
						return
					}
				}
			}
		}
	}()

	select {
	case err := <-gotTerminated:
		require.NoError(t, err)
	case <-time.After(60 * time.Second):
		var sb strings.Builder
		_ = pprof.Lookup("goroutine").WriteTo(&sb, 2)
		var stuck []string
		for _, g := range strings.Split(sb.String(), "\n\n") {
			if strings.Contains(g, "peerstream.(*Server).realHandleStream") {
				stuck = append(stuck, g)
			}
		}
		t.Fatalf("peer never received Terminated after local deletion\n\nrealHandleStream goroutines:\n%s", strings.Join(stuck, "\n\n"))
	}

	require.Eventually(t, func() bool {
		_, found := acceptor.StreamStatus(p.ID)
		return !found
	}, 10*time.Second, 50*time.Millisecond, "stream status was not deleted")
}
