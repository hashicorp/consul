package consul

import (
	"container/ring"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/pbpeering"
)

func (s *Server) startPeeringStreamSync(ctx context.Context) {
	s.leaderRoutineManager.Start(ctx, peeringStreamsRoutineName, s.runPeeringSync)
}

func (s *Server) runPeeringSync(ctx context.Context) error {
	logger := s.logger.Named("peering-syncer")
	cancelFns := make(map[string]context.CancelFunc)

	retryLoopBackoff(ctx, func() error {
		if err := s.syncPeeringsAndBlock(ctx, logger, cancelFns); err != nil {
			return err
		}
		return nil

	}, func(err error) {
		s.logger.Error("error syncing peering streams from state store", "error", err)
	})

	return nil
}

func (s *Server) stopPeeringStreamSync() {
	// will be a no-op when not started
	s.leaderRoutineManager.Stop(peeringStreamsRoutineName)
}

// syncPeeringsAndBlock is a long-running goroutine that is responsible for watching
// changes to peerings in the state store and managing streams to those peers.
func (s *Server) syncPeeringsAndBlock(ctx context.Context, logger hclog.Logger, cancelFns map[string]context.CancelFunc) error {
	state := s.fsm.State()

	// Pull the state store contents and set up to block for changes.
	ws := memdb.NewWatchSet()
	ws.Add(state.AbandonCh())
	ws.Add(ctx.Done())

	_, peers, err := state.PeeringList(ws, *structs.NodeEnterpriseMetaInPartition(structs.WildcardSpecifier))
	if err != nil {
		return err
	}

	// TODO(peering) Adjust this debug info.
	// Generate a UUID to trace different passes through this function.
	seq, err := uuid.GenerateUUID()
	if err != nil {
		s.logger.Debug("failed to generate sequence uuid while syncing peerings")
	}

	logger.Trace("syncing new list of peers", "num_peers", len(peers), "sequence_id", seq)

	// Stored tracks the unique set of peers that should be dialed.
	// It is used to reconcile the list of active streams.
	stored := make(map[string]struct{})

	var merr *multierror.Error

	// Create connections and streams to peers in the state store that do not have an active stream.
	for _, peer := range peers {
		logger.Trace("evaluating stored peer", "peer", peer.Name, "should_dial", peer.ShouldDial(), "sequence_id", seq)

		if !peer.ShouldDial() {
			continue
		}

		// TODO(peering) Account for deleted peers that are still in the state store
		stored[peer.ID] = struct{}{}

		status, found := s.peeringService.StreamStatus(peer.ID)

		// TODO(peering): If there is new peering data and a connected stream, should we tear down the stream?
		//                If the data in the updated token is bad, the user wouldn't know until the old servers/certs become invalid.
		//                Alternatively we could do a basic Ping from the initiate peering endpoint to avoid dealing with that here.
		if found && status.Connected {
			// Nothing to do when we already have an active stream to the peer.
			continue
		}
		logger.Trace("ensuring stream to peer", "peer_id", peer.ID, "sequence_id", seq)

		if cancel, ok := cancelFns[peer.ID]; ok {
			// If the peer is known but we're not connected, clean up the retry-er and start over.
			// There may be new data in the state store that would enable us to get out of an error state.
			logger.Trace("cancelling context to re-establish stream", "peer_id", peer.ID, "sequence_id", seq)
			cancel()
		}

		if err := s.establishStream(ctx, logger, peer, cancelFns); err != nil {
			// TODO(peering): These errors should be reported in the peer status, otherwise they're only in the logs.
			//                Lockable status isn't available here though. Could report it via the peering.Service?
			logger.Error("error establishing peering stream", "peer_id", peer.ID, "error", err)
			merr = multierror.Append(merr, err)

			// Continue on errors to avoid one bad peering from blocking the establishment and cleanup of others.
			continue
		}
	}

	logger.Trace("checking connected streams", "streams", s.peeringService.ConnectedStreams(), "sequence_id", seq)

	// Clean up active streams of peerings that were deleted from the state store.
	// TODO(peering): This is going to trigger shutting down peerings we generated a token for. Is that OK?
	for stream, doneCh := range s.peeringService.ConnectedStreams() {
		if _, ok := stored[stream]; ok {
			// Active stream is in the state store, nothing to do.
			continue
		}

		select {
		case <-doneCh:
			// channel is closed, do nothing to avoid a panic
		default:
			logger.Trace("tearing down stream for deleted peer", "peer_id", stream, "sequence_id", seq)
			close(doneCh)
		}
	}

	logger.Trace("blocking for changes", "sequence_id", seq)

	// Block for any changes to the state store.
	ws.WatchCtx(ctx)

	logger.Trace("unblocked", "sequence_id", seq)
	return merr.ErrorOrNil()
}

func (s *Server) establishStream(ctx context.Context, logger hclog.Logger, peer *pbpeering.Peering, cancelFns map[string]context.CancelFunc) error {
	tlsOption := grpc.WithInsecure()
	if len(peer.PeerCAPems) > 0 {
		var haveCerts bool
		pool := x509.NewCertPool()
		for _, pem := range peer.PeerCAPems {
			if !pool.AppendCertsFromPEM([]byte(pem)) {
				return fmt.Errorf("failed to parse PEM %s", pem)
			}
			if len(pem) > 0 {
				haveCerts = true
			}
		}
		if !haveCerts {
			return fmt.Errorf("failed to build cert pool from peer CA pems")
		}
		cfg := tls.Config{
			ServerName: peer.PeerServerName,
			RootCAs:    pool,
		}
		tlsOption = grpc.WithTransportCredentials(credentials.NewTLS(&cfg))
	}

	// Create a ring buffer to cycle through peer addresses in the retry loop below.
	buffer := ring.New(len(peer.PeerServerAddresses))
	for _, addr := range peer.PeerServerAddresses {
		buffer.Value = addr
		buffer = buffer.Next()
	}

	logger.Trace("establishing stream to peer", "peer_id", peer.ID)

	retryCtx, cancel := context.WithCancel(ctx)
	cancelFns[peer.ID] = cancel

	// Establish a stream-specific retry so that retrying stream/conn errors isn't dependent on state store changes.
	go retryLoopBackoff(retryCtx, func() error {
		// Try a new address on each iteration by advancing the ring buffer on errors.
		defer func() {
			buffer = buffer.Next()
		}()
		addr, ok := buffer.Value.(string)
		if !ok {
			return fmt.Errorf("peer server address type %T is not a string", buffer.Value)
		}

		logger.Trace("dialing peer", "peer_id", peer.ID, "addr", addr)
		conn, err := grpc.DialContext(retryCtx, addr,
			grpc.WithContextDialer(newPeerDialer(addr)),
			grpc.WithBlock(),
			tlsOption,
		)
		if err != nil {
			return fmt.Errorf("failed to dial: %w", err)
		}
		defer conn.Close()

		client := pbpeering.NewPeeringServiceClient(conn)
		stream, err := client.StreamResources(retryCtx)
		if err != nil {
			return err
		}

		err = s.peeringService.HandleStream(peering.HandleStreamRequest{
			LocalID:   peer.ID,
			RemoteID:  peer.PeerID,
			PeerName:  peer.Name,
			Partition: peer.Partition,
			Stream:    stream,
		})
		if err == nil {
			// This will cancel the retry-er context, letting us break out of this loop when we want to shut down the stream.
			cancel()
		}
		return err

	}, func(err error) {
		// TODO(peering): These errors should be reported in the peer status, otherwise they're only in the logs.
		//                Lockable status isn't available here though. Could report it via the peering.Service?
		logger.Error("error managing peering stream", "peer_id", peer.ID, "error", err)
	})

	return nil
}

func newPeerDialer(peerAddr string) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, addr string) (net.Conn, error) {
		d := net.Dialer{}
		conn, err := d.DialContext(ctx, "tcp", peerAddr)
		if err != nil {
			return nil, err
		}

		// TODO(peering): This is going to need to be revisited. This type uses the TLS settings configured on the agent, but
		//                for peering we never want mutual TLS because the client peer doesn't share its CA cert.
		_, err = conn.Write([]byte{byte(pool.RPCGRPC)})
		if err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	}
}
