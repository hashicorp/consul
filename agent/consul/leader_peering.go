package consul

import (
	"container/ring"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-uuid"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/rpc/peering"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logging"
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
	// We have to be careful not to introduce a data race here. We want to
	// compare the current known peerings in the state store with known
	// connected streams to know when we should TERMINATE stray peerings.
	//
	// If you read the current peerings from the state store, then read the
	// current established streams you could lose the data race and have the
	// sequence of events be:
	//
	//   1. list peerings [A,B,C]
	//   2. persist new peering [D]
	//   3. accept new stream for [D]
	//   4. list streams [A,B,C,D]
	//   5. terminate [D]
	//
	// Which is wrong. If we instead ensure that (4) happens before (1), given
	// that you can't get an established stream without first passing a "does
	// this peering exist in the state store?" inquiry then this happens:
	//
	//   1. list streams [A,B,C]
	//   2. list peerings [A,B,C]
	//   3. persist new peering [D]
	//   4. accept new stream for [D]
	//   5. terminate []
	//
	// Or even this is fine:
	//
	//   1. list streams [A,B,C]
	//   2. persist new peering [D]
	//   3. accept new stream for [D]
	//   4. list peerings [A,B,C,D]
	//   5. terminate []
	connectedStreams := s.peeringService.ConnectedStreams()

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

		if !peer.IsActive() {
			// The peering was marked for deletion by ourselves or our peer, no need to dial or track them.
			continue
		}

		// Track all active peerings,since the reconciliation loop below applies to the token generator as well.
		stored[peer.ID] = struct{}{}

		if !peer.ShouldDial() {
			// We do not need to dial peerings where we generated the peering token.
			continue
		}

		status, found := s.peeringService.StreamStatus(peer.ID)

		// TODO(peering): If there is new peering data and a connected stream, should we tear down the stream?
		//                If the data in the updated token is bad, the user wouldn't know until the old servers/certs become invalid.
		//                Alternatively we could do a basic Ping from the establish peering endpoint to avoid dealing with that here.
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
	for stream, doneCh := range connectedStreams {
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
	logger = logger.With("peer_name", peer.Name, "peer_id", peer.ID)

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

	logger.Trace("establishing stream to peer")

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

		logger.Trace("dialing peer", "addr", addr)
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

		streamReq := peering.HandleStreamRequest{
			LocalID:   peer.ID,
			RemoteID:  peer.PeerID,
			PeerName:  peer.Name,
			Partition: peer.Partition,
			Stream:    stream,
		}
		err = s.peeringService.HandleStream(streamReq)
		// A nil error indicates that the peering was deleted and the stream needs to be gracefully shutdown.
		if err == nil {
			stream.CloseSend()
			s.peeringService.DrainStream(streamReq)

			// This will cancel the retry-er context, letting us break out of this loop when we want to shut down the stream.
			cancel()

			logger.Info("closed outbound stream")
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

func (s *Server) startPeeringDeferredDeletion(ctx context.Context) {
	s.leaderRoutineManager.Start(ctx, peeringDeletionRoutineName, s.runPeeringDeletions)
}

// runPeeringDeletions watches for peerings marked for deletions and then cleans up data for them.
func (s *Server) runPeeringDeletions(ctx context.Context) error {
	logger := s.loggers.Named(logging.Peering)

	// This limiter's purpose is to control the rate of raft applies caused by the deferred deletion
	// process. This includes deletion of the peerings themselves in addition to any peering data
	raftLimiter := rate.NewLimiter(defaultDeletionApplyRate, int(defaultDeletionApplyRate))
	for {
		ws := memdb.NewWatchSet()
		state := s.fsm.State()
		_, peerings, err := s.fsm.State().PeeringListDeleted(ws)
		if err != nil {
			logger.Warn("encountered an error while searching for deleted peerings", "error", err)
			continue
		}

		if len(peerings) == 0 {
			ws.Add(state.AbandonCh())

			// wait for a peering to be deleted or the routine to be cancelled
			if err := ws.WatchCtx(ctx); err != nil {
				return err
			}
			continue
		}

		for _, p := range peerings {
			s.removePeeringAndData(ctx, logger, raftLimiter, p)
		}
	}
}

// removepPeeringAndData removes data imported for a peering and the peering itself.
func (s *Server) removePeeringAndData(ctx context.Context, logger hclog.Logger, limiter *rate.Limiter, peer *pbpeering.Peering) {
	logger = logger.With("peer_name", peer.Name, "peer_id", peer.ID)
	entMeta := *structs.NodeEnterpriseMetaInPartition(peer.Partition)

	// First delete all imported data.
	// By deleting all imported nodes we also delete all services and checks registered on them.
	if err := s.deleteAllNodes(ctx, limiter, entMeta, peer.Name); err != nil {
		logger.Error("Failed to remove Nodes for peer", "error", err)
		return
	}
	if err := s.deleteTrustBundleFromPeer(ctx, limiter, entMeta, peer.Name); err != nil {
		logger.Error("Failed to remove trust bundle for peer", "error", err)
		return
	}

	if err := limiter.Wait(ctx); err != nil {
		return
	}

	if peer.State == pbpeering.PeeringState_TERMINATED {
		// For peerings terminated by our peer we only clean up the local data, we do not delete the peering itself.
		// This is to avoid a situation where the peering disappears without the local operator's knowledge.
		return
	}

	// Once all imported data is deleted, the peering itself is also deleted.
	req := &pbpeering.PeeringDeleteRequest{
		Name:      peer.Name,
		Partition: acl.PartitionOrDefault(peer.Partition),
	}
	_, err := s.raftApplyProtobuf(structs.PeeringDeleteType, req)
	if err != nil {
		logger.Error("failed to apply full peering deletion", "error", err)
		return
	}
}

// deleteAllNodes will delete all nodes in a partition or all nodes imported from a given peer name.
func (s *Server) deleteAllNodes(ctx context.Context, limiter *rate.Limiter, entMeta acl.EnterpriseMeta, peerName string) error {
	// Same as ACL batch upsert size
	nodeBatchSizeBytes := 256 * 1024

	_, nodes, err := s.fsm.State().NodeDump(nil, &entMeta, peerName)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return nil
	}

	i := 0
	for {
		var ops structs.TxnOps
		for batchSize := 0; batchSize < nodeBatchSizeBytes && i < len(nodes); i++ {
			entry := nodes[i]

			op := structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: api.NodeDelete,
					Node: structs.Node{
						Node:      entry.Node,
						Partition: entry.Partition,
						PeerName:  entry.PeerName,
					},
				},
			}
			ops = append(ops, &op)

			// Add entries to the transaction until it reaches the max batch size
			batchSize += len(entry.Node) + len(entry.Partition) + len(entry.PeerName)
		}

		// Send each batch as a TXN Req to avoid sending one at a time
		req := structs.TxnRequest{
			Datacenter: s.config.Datacenter,
			Ops:        ops,
		}
		if len(req.Ops) > 0 {
			if err := limiter.Wait(ctx); err != nil {
				return err
			}

			_, err := s.raftApplyMsgpack(structs.TxnRequestType, &req)
			if err != nil {
				return err
			}
		} else {
			break
		}
	}

	return nil
}

// deleteTrustBundleFromPeer deletes the trust bundle imported from a peer, if present.
func (s *Server) deleteTrustBundleFromPeer(ctx context.Context, limiter *rate.Limiter, entMeta acl.EnterpriseMeta, peerName string) error {
	_, bundle, err := s.fsm.State().PeeringTrustBundleRead(nil, state.Query{Value: peerName, EnterpriseMeta: entMeta})
	if err != nil {
		return err
	}
	if bundle == nil {
		return nil
	}

	if err := limiter.Wait(ctx); err != nil {
		return err
	}

	req := &pbpeering.PeeringTrustBundleDeleteRequest{
		Name:      peerName,
		Partition: entMeta.PartitionOrDefault(),
	}
	_, err = s.raftApplyProtobuf(structs.PeeringTrustBundleDeleteType, req)
	return err
}
