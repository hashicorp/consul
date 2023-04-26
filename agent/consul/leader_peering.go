// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"container/ring"
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/go-uuid"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/grpc-external/services/peerstream"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/private/pbcommon"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/proto/private/pbpeerstream"
)

var leaderExportedServicesCountKeyDeprecated = []string{"consul", "peering", "exported_services"}
var leaderExportedServicesCountKey = []string{"peering", "exported_services"}
var leaderHealthyPeeringKeyDeprecated = []string{"consul", "peering", "healthy"}
var leaderHealthyPeeringKey = []string{"peering", "healthy"}
var LeaderPeeringMetrics = []prometheus.GaugeDefinition{
	{
		Name: leaderExportedServicesCountKeyDeprecated,
		Help: fmt.Sprint("Deprecated - please use ", strings.Join(leaderExportedServicesCountKey, "_")),
	},
	{
		Name: leaderExportedServicesCountKey,
		Help: "A gauge that tracks how many services are exported for the peering. " +
			"The labels are \"peer_name\", \"peer_id\" and, for enterprise, \"partition\". " +
			"We emit this metric every 9 seconds",
	},
	{
		Name: leaderHealthyPeeringKeyDeprecated,
		Help: fmt.Sprint("Deprecated - please use ", strings.Join(leaderExportedServicesCountKey, "_")),
	},
	{
		Name: leaderHealthyPeeringKey,
		Help: "A gauge that tracks how if a peering is healthy (1) or not (0). " +
			"The labels are \"peer_name\", \"peer_id\" and, for enterprise, \"partition\". " +
			"We emit this metric every 9 seconds",
	},
}
var (
	// fastConnRetryTimeout is how long we wait between retrying connections following the "fast" path
	// which is triggered on specific connection errors.
	fastConnRetryTimeout = 8 * time.Millisecond
	// maxFastConnRetries is the maximum number of fast connection retries before we follow exponential backoff.
	maxFastConnRetries = uint(5)
	// maxFastRetryBackoff is the maximum amount of time we'll wait between retries following the fast path.
	maxFastRetryBackoff = 8192 * time.Millisecond
	// maxRetryBackoffPeering is the maximum number of seconds we'll wait between retries when attempting to re-establish a peering connection.
	maxRetryBackoffPeering = 64
)

func (s *Server) startPeeringStreamSync(ctx context.Context) {
	s.leaderRoutineManager.Start(ctx, peeringStreamsRoutineName, s.runPeeringSync)
	s.leaderRoutineManager.Start(ctx, peeringStreamsMetricsRoutineName, s.runPeeringMetrics)
}

func (s *Server) runPeeringMetrics(ctx context.Context) error {
	ticker := time.NewTicker(s.config.MetricsReportingInterval)
	defer ticker.Stop()

	logger := s.logger.Named(logging.PeeringMetrics)
	defaultMetrics := metrics.Default

	for {
		select {
		case <-ctx.Done():
			logger.Info("stopping peering metrics")

			// "Zero-out" the metric on exit so that when prometheus scrapes this
			// metric from a non-leader, it does not get a stale value.
			metrics.SetGauge(leaderExportedServicesCountKeyDeprecated, float32(0))
			metrics.SetGauge(leaderExportedServicesCountKey, float32(0))
			return nil
		case <-ticker.C:
			if err := s.emitPeeringMetricsOnce(defaultMetrics()); err != nil {
				s.logger.Error("error emitting peering stream metrics", "error", err)
			}
		}
	}
}

func (s *Server) emitPeeringMetricsOnce(metricsImpl *metrics.Metrics) error {
	_, peers, err := s.fsm.State().PeeringList(nil, *structs.NodeEnterpriseMetaInPartition(structs.WildcardSpecifier))
	if err != nil {
		return err
	}

	for _, peer := range peers {
		part := peer.Partition
		labels := []metrics.Label{
			{Name: "peer_name", Value: peer.Name},
			{Name: "peer_id", Value: peer.ID},
		}
		if part != "" {
			labels = append(labels, metrics.Label{Name: "partition", Value: part})
		}

		status, found := s.peerStreamServer.StreamStatus(peer.ID)
		if found {
			// exported services count metric
			esc := status.GetExportedServicesCount()
			metricsImpl.SetGaugeWithLabels(leaderExportedServicesCountKeyDeprecated, float32(esc), labels)
			metricsImpl.SetGaugeWithLabels(leaderExportedServicesCountKey, float32(esc), labels)
		}

		// peering health metric
		healthy := 0
		switch {
		case status.NeverConnected:
		case s.peerStreamServer.Tracker.IsHealthy(status):
			healthy = 1
		}
		metricsImpl.SetGaugeWithLabels(leaderHealthyPeeringKeyDeprecated, float32(healthy), labels)
		metricsImpl.SetGaugeWithLabels(leaderHealthyPeeringKey, float32(healthy), labels)
	}

	return nil
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
	s.leaderRoutineManager.Stop(peeringStreamsMetricsRoutineName)
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
	connectedStreams := s.peerStreamServer.ConnectedStreams()

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

		// We may have written this peering to the store to trigger xDS updates, but still in the process of establishing.
		// If there isn't a secret yet, we're still trying to reach the other server.
		logger.Trace("reading peering secret", "sequence_id", seq)
		secret, err := s.fsm.State().PeeringSecretsRead(ws, peer.ID)
		if err != nil {
			return fmt.Errorf("failed to read secret for peering: %w", err)
		}
		if secret.GetStream().GetActiveSecretID() == "" {
			continue
		}

		status, found := s.peerStreamServer.StreamStatus(peer.ID)
		if found && status.Connected {
			// Nothing to do when we already have an active stream to the peer.
			// Updated data will only be used if the stream becomes disconnected
			// since there's no need to tear down an active stream.
			continue
		}
		logger.Trace("ensuring stream to peer", "peer_id", peer.ID, "sequence_id", seq)

		if cancel, ok := cancelFns[peer.ID]; ok {
			// If the peer is known but we're not connected, clean up the retry-er and start over.
			// There may be new data in the state store that would enable us to get out of an error state.
			logger.Trace("cancelling context to re-establish stream", "peer_id", peer.ID, "sequence_id", seq)
			cancel()
		}

		if err := s.establishStream(ctx, logger, peer, secret, cancelFns); err != nil {
			// TODO(peering): These errors should be reported in the peer status, otherwise they're only in the logs.
			//                Lockable status isn't available here though. Could report it via the peering.Service?
			logger.Error("error establishing peering stream", "peer_id", peer.ID, "error", err)
			merr = multierror.Append(merr, err)

			// Continue on errors to avoid one bad peering from blocking the establishment and cleanup of others.
			continue
		}
	}

	logger.Trace("checking connected streams", "streams", connectedStreams, "sequence_id", seq)

	// Clean up active streams of peerings that were deleted from the state store.
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

func (s *Server) establishStream(ctx context.Context,
	logger hclog.Logger,
	peer *pbpeering.Peering,
	secret *pbpeering.PeeringSecrets,
	cancelFns map[string]context.CancelFunc) error {
	logger = logger.With("peer_name", peer.Name, "peer_id", peer.ID)

	if peer.PeerID == "" {
		return fmt.Errorf("expected PeerID to be non empty; the wrong end of peering is being dialed")
	}

	tlsOption, err := peer.TLSDialOption()
	if err != nil {
		return fmt.Errorf("failed to build TLS dial option from peering: %w", err)
	}

	if secret.GetStream().GetActiveSecretID() == "" {
		return errors.New("missing stream secret for peering stream authorization, peering must be re-established")
	}

	logger.Trace("establishing stream to peer")

	streamStatus, err := s.peerStreamServer.Tracker.Register(peer.ID)
	if err != nil {
		return fmt.Errorf("failed to register stream: %v", err)
	}

	streamCtx, cancel := context.WithCancel(ctx)
	cancelFns[peer.ID] = cancel

	// Start a goroutine to watch for updates to peer server addresses.
	// The latest valid server address can be received from nextServerAddr.
	nextServerAddr := make(chan string)
	go s.watchAddresses(streamCtx, peer.ID, nextServerAddr)

	// Establish a stream-specific retry so that retrying stream/conn errors isn't dependent on state store changes.
	go retryLoopBackoffPeering(streamCtx, logger, func() error {
		// Try a new address on each iteration by advancing the ring buffer on errors.
		addr, stillOpen := <-nextServerAddr
		if !stillOpen {
			// If the channel was closed that means the context was canceled, so we return.
			return streamCtx.Err()
		}

		opts := []grpc.DialOption{
			tlsOption,
			// TODO(peering): Use a grpc.WithStatsHandler here.
			//				  This should wait until the grpc-external server is wired up with a stats handler in NET-50.
			// For keep alive parameters there is a larger comment in ClientConnPool.dial about that.
			grpc.WithKeepaliveParams(keepalive.ClientParameters{
				Time:    30 * time.Second,
				Timeout: 10 * time.Second,
				// send keepalive pings even if there is no active streams
				PermitWithoutStream: true,
			}),
			grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(8*1024*1024), grpc.MaxCallRecvMsgSize(8*1024*1024)),
		}

		logger.Trace("dialing peer", "addr", addr)
		conn, err := grpc.DialContext(streamCtx, addr, opts...)

		if err != nil {
			return fmt.Errorf("failed to dial: %w", err)
		}
		defer conn.Close()

		client := pbpeerstream.NewPeerStreamServiceClient(conn)
		stream, err := client.StreamResources(streamCtx)
		if err != nil {
			return err
		}

		initialReq := &pbpeerstream.ReplicationMessage{
			Payload: &pbpeerstream.ReplicationMessage_Open_{
				Open: &pbpeerstream.ReplicationMessage_Open{
					PeerID:         peer.PeerID,
					StreamSecretID: secret.GetStream().GetActiveSecretID(),
					Remote: &pbpeering.RemoteInfo{
						Partition:  peer.Partition,
						Datacenter: s.config.Datacenter,
						Locality:   pbcommon.LocalityToProto(s.config.Locality),
					},
				},
			},
		}
		if err := stream.Send(initialReq); err != nil {
			return fmt.Errorf("failed to send initial stream request: %w", err)
		}

		streamReq := peerstream.HandleStreamRequest{
			LocalID:   peer.ID,
			RemoteID:  peer.PeerID,
			PeerName:  peer.Name,
			Partition: peer.Partition,
			Stream:    stream,
		}
		err = s.peerStreamServer.HandleStream(streamReq)
		// A nil error indicates that the peering was deleted and the stream needs to be gracefully shutdown.
		if err == nil {
			stream.CloseSend()
			s.peerStreamServer.DrainStream(streamReq)
			cancel()
			logger.Info("closed outbound stream")
		}
		return err

	}, func(err error) {
		// TODO(peering): why are we using TrackSendError here? This could also be a receive error.
		streamStatus.TrackSendError(err.Error())

		switch {
		case isErrCode(err, codes.FailedPrecondition):
			logger.Debug("stream disconnected due to 'failed precondition' error; reconnecting",
				"error", err)

		case isErrCode(err, codes.ResourceExhausted):
			logger.Debug("stream disconnected due to 'resource exhausted' error; reconnecting",
				"error", err)

		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			logger.Debug("stream context was canceled", "error", err)

		case err != nil:
			logger.Error("error managing peering stream", "error", err)
		}
	}, peeringRetryTimeout)

	return nil
}

// watchAddresses sends an up-to-date address to nextServerAddr.
// These could be either remote peer server addresses, or local mesh gateways.
// The function loads the addresses into a ring buffer and cycles through them until:
//  1. streamCtx is cancelled (peer is deleted or we're re-establishing the stream with new data)
//  2. the peer, Mesh config entry, or (optionally) mesh gateway address set is modified, and the watchset fires.
//
// In case (2) we re-fetch all the data sources and rebuild the ring buffer.
// In the event that the PeerThroughMeshGateways is set in the Mesh entry, we front-load the ring buffer with
// local mesh gateway addresses, so we can try those first, with the option to fall back to remote server addresses.
func (s *Server) watchAddresses(ctx context.Context, peerID string, nextServerAddr chan<- string) {
	defer close(nextServerAddr)

	var ringbuf *ring.Ring
	var ws memdb.WatchSet

	fetchAddresses := func() error {
		// Re-instantiate ws since it can only be watched once.
		ws = memdb.NewWatchSet()

		newRing, _, err := s.peeringBackend.GetDialAddresses(s.logger, ws, peerID)
		if err != nil {
			return fmt.Errorf("failed to fetch updated addresses to dial peer: %w", err)
		}
		ringbuf = newRing

		return nil
	}

	// Initialize the first ring buffer.
	if err := fetchAddresses(); err != nil {
		s.logger.Warn("error fetching addresses", "peer_id", peerID, "error", err)
	}

	for {
		select {
		case nextServerAddr <- ringbuf.Value.(string):
			ringbuf = ringbuf.Next()

		case err := <-ws.WatchCh(ctx):
			if err != nil {
				// Context was cancelled.
				return
			}

			// Watch fired so we re-fetch the necessary addresses and replace the ring buffer.
			if err := fetchAddresses(); err != nil {
				s.logger.Warn("watch for new addresses fired but the address list to dial may not have been updated",
					"peer_id", peerID,
					"error", err)
			}
		}
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
		select {
		case <-ctx.Done():
			return nil
		default:
		}

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

// retryLoopBackoffPeering re-runs loopFn with a backoff on error. errFn is run whenever
// loopFn returns an error. retryTimeFn is used to calculate the time between retries on error.
// It is passed the number of errors in a row that loopFn has returned and the latest error
// from loopFn.
//
// This function is modelled off of retryLoopBackoffHandleSuccess but is specific to peering
// because peering needs to use different retry times depending on which error is returned.
// This function doesn't use a rate limiter, unlike retryLoopBackoffHandleSuccess, because
// the rate limiter is only needed in the success case when loopFn returns nil and we want to
// loop again. In the peering case, we exit on a successful loop so we don't need the limter.
func retryLoopBackoffPeering(ctx context.Context, logger hclog.Logger, loopFn func() error, errFn func(error),
	retryTimeFn func(failedAttempts uint, loopErr error) time.Duration) {
	var failedAttempts uint
	var err error
	for {
		if err = loopFn(); err != nil {
			errFn(err)

			if failedAttempts < math.MaxUint {
				failedAttempts++
			}

			retryTime := retryTimeFn(failedAttempts, err)
			logger.Trace("in connection retry backoff", "delay", retryTime)
			timer := time.NewTimer(retryTime)

			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			continue
		}
		return
	}
}

// peeringRetryTimeout returns the time that should be waited between re-establishing a peering
// connection after an error. We follow the default backoff from retryLoopBackoff
// unless the error is a "failed precondition" error in which case we retry much more quickly.
// Retrying quickly is important in the case of a failed precondition error because we expect it to resolve
// quickly. For example in the case of connecting with a follower through a load balancer, we just need to retry
// until our request lands on a leader.
func peeringRetryTimeout(failedAttempts uint, loopErr error) time.Duration {
	if loopErr != nil && isErrCode(loopErr, codes.FailedPrecondition) {
		// Wait a constant time for the first number of retries.
		if failedAttempts <= maxFastConnRetries {
			return fastConnRetryTimeout
		}
		// From here, follow an exponential backoff maxing out at maxFastRetryBackoff.
		// The below equation multiples the constantRetryTimeout by 2^n where n is the number of failed attempts
		// we're on, starting at 1 now that we're past our maxFastConnRetries.
		// For example if fastConnRetryTimeout == 8ms and maxFastConnRetries == 5, then at 6 failed retries
		// we'll do 8ms * 2^1 = 16ms, then 8ms * 2^2 = 32ms, etc.
		ms := fastConnRetryTimeout * (1 << (failedAttempts - maxFastConnRetries))
		if ms > maxFastRetryBackoff {
			return maxFastRetryBackoff
		}
		return ms
	}

	// if the message sent is too large probably should not retry at all
	if loopErr != nil && isErrCode(loopErr, codes.ResourceExhausted) {
		return maxFastRetryBackoff
	}

	// Else we go with the default backoff from retryLoopBackoff.
	if (1 << failedAttempts) < maxRetryBackoffPeering {
		return (1 << failedAttempts) * time.Second
	}
	return time.Duration(maxRetryBackoffPeering) * time.Second
}

// isErrCode returns true if err is a gRPC error with given error code.
func isErrCode(err error, code codes.Code) bool {
	if err == nil {
		return false
	}
	// Handle wrapped errors, since status.FromError does a naive assertion.
	var statusErr interface {
		GRPCStatus() *grpcstatus.Status
	}
	if errors.As(err, &statusErr) {
		return statusErr.GRPCStatus().Code() == code
	}

	grpcErr, ok := grpcstatus.FromError(err)
	if !ok {
		return false
	}
	return grpcErr.Code() == code
}
