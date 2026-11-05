// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package peerstream

import (
	"context"
	"fmt"
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
