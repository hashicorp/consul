// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"time"

	"github.com/armon/go-metrics"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/consul/wanfed"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	libserf "github.com/hashicorp/consul/lib/serf"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/types"
)

const (
	// StatusReap is used to update the status of a node if we
	// are handling a EventMemberReap
	StatusReap = serf.MemberStatus(-1)

	// userEventPrefix is pre-pended to a user event to distinguish it
	userEventPrefix = "consul:event:"

	// maxPeerRetries limits how many invalidate attempts are made
	maxPeerRetries = 6
)

type setupSerfOptions struct {
	Config       *serf.Config
	EventCh      chan serf.Event
	SnapshotPath string
	Listener     net.Listener

	// WAN only
	WAN bool

	// LAN only
	Segment   string
	Partition string
}

// setupSerf is used to setup and initialize a Serf
func (s *Server) setupSerf(opts setupSerfOptions) (*serf.Serf, *serf.Config, error) {
	conf, err := s.setupSerfConfig(opts)
	if err != nil {
		return nil, nil, err
	}

	cluster, err := serf.Create(conf)
	if err != nil {
		return nil, nil, err
	}

	return cluster, conf, nil
}

func (s *Server) setupSerfConfig(opts setupSerfOptions) (*serf.Config, error) {
	if opts.Config == nil {
		return nil, errors.New("serf config is a required field")
	}
	if opts.Listener == nil {
		return nil, errors.New("listener is a required field")
	}
	if opts.WAN {
		if opts.Segment != "" {
			return nil, errors.New("cannot configure segments on the WAN serf pool")
		}
		if opts.Partition != "" {
			return nil, errors.New("cannot configure partitions on the WAN serf pool")
		}
	}

	conf := opts.Config
	conf.Init()

	if opts.WAN {
		conf.NodeName = fmt.Sprintf("%s.%s", s.config.NodeName, s.config.Datacenter)
	} else {
		conf.NodeName = s.config.NodeName
		if s.config.SerfWANConfig != nil {
			serfBindPortWAN := s.config.SerfWANConfig.MemberlistConfig.BindPort
			if serfBindPortWAN > 0 {
				conf.Tags["wan_join_port"] = fmt.Sprintf("%d", serfBindPortWAN)
			}
		}
	}
	conf.Tags["role"] = "consul"
	conf.Tags["dc"] = s.config.Datacenter
	conf.Tags["segment"] = opts.Segment
	conf.Tags["id"] = string(s.config.NodeID)
	conf.Tags["vsn"] = fmt.Sprintf("%d", s.config.ProtocolVersion)
	conf.Tags["vsn_min"] = fmt.Sprintf("%d", ProtocolVersionMin)
	conf.Tags["vsn_max"] = fmt.Sprintf("%d", ProtocolVersionMax)
	conf.Tags["raft_vsn"] = fmt.Sprintf("%d", s.config.RaftConfig.ProtocolVersion)
	conf.Tags["build"] = s.config.Build
	addr := opts.Listener.Addr().(*net.TCPAddr)
	conf.Tags["port"] = fmt.Sprintf("%d", addr.Port)
	if s.config.GRPCPort > 0 {
		conf.Tags["grpc_port"] = fmt.Sprintf("%d", s.config.GRPCPort)
	}
	if s.config.GRPCTLSPort > 0 {
		conf.Tags["grpc_tls_port"] = fmt.Sprintf("%d", s.config.GRPCTLSPort)
	}
	if s.config.Bootstrap {
		conf.Tags["bootstrap"] = "1"
	}
	if s.config.BootstrapExpect != 0 {
		conf.Tags["expect"] = fmt.Sprintf("%d", s.config.BootstrapExpect)
	}
	if s.config.ReadReplica {
		// DEPRECATED - This tag should be removed when we no longer want to support
		// upgrades from 1.8.x and below
		conf.Tags["nonvoter"] = "1"
		conf.Tags["read_replica"] = "1"
	}
	if s.config.TLSConfig.InternalRPC.CAPath != "" || s.config.TLSConfig.InternalRPC.CAFile != "" {
		conf.Tags["use_tls"] = "1"
	}

	// TODO(ACL-Legacy-Compat): remove in phase 2. These are kept for now to
	// allow for upgrades.
	if s.ACLResolver.ACLsEnabled() {
		conf.Tags[metadata.TagACLs] = string(structs.ACLModeEnabled)
	} else {
		conf.Tags[metadata.TagACLs] = string(structs.ACLModeDisabled)
	}

	// feature flag: advertise support for federation states
	conf.Tags["ft_fs"] = "1"

	// feature flag: advertise support for service-intentions
	conf.Tags["ft_si"] = "1"

	var subLoggerName string
	if opts.WAN {
		subLoggerName = logging.WAN
	} else {
		subLoggerName = logging.LAN
	}

	// Wrap hclog in a standard logger wrapper for serf and memberlist
	// We use the Intercept variant here to ensure that serf and memberlist logs
	// can be streamed via the monitor endpoint
	serfLogger := s.logger.
		NamedIntercept(logging.Serf).
		NamedIntercept(subLoggerName).
		StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true})
	memberlistLogger := s.logger.
		NamedIntercept(logging.Memberlist).
		NamedIntercept(subLoggerName).
		StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true})

	conf.MemberlistConfig.Logger = memberlistLogger
	conf.Logger = serfLogger
	conf.EventCh = opts.EventCh
	conf.ProtocolVersion = protocolVersionMap[s.config.ProtocolVersion]
	conf.RejoinAfterLeave = s.config.RejoinAfterLeave
	if opts.WAN {
		conf.Merge = &wanMergeDelegate{
			localDatacenter: s.config.Datacenter,
		}
	} else {
		conf.Merge = &lanMergeDelegate{
			dc:        s.config.Datacenter,
			nodeID:    s.config.NodeID,
			nodeName:  s.config.NodeName,
			segment:   opts.Segment,
			partition: opts.Partition,
			server:    true,
		}
	}

	if opts.WAN {
		nt, err := memberlist.NewNetTransport(&memberlist.NetTransportConfig{
			BindAddrs:    []string{conf.MemberlistConfig.BindAddr},
			BindPort:     conf.MemberlistConfig.BindPort,
			Logger:       conf.MemberlistConfig.Logger,
			MetricLabels: []metrics.Label{{Name: "network", Value: "wan"}},
		})
		if err != nil {
			return nil, err
		}

		if s.config.ConnectMeshGatewayWANFederationEnabled {
			mgwTransport, err := wanfed.NewTransport(
				s.tlsConfigurator,
				nt,
				s.config.Datacenter,
				s.gatewayLocator.PickGateway,
			)
			if err != nil {
				return nil, err
			}

			conf.MemberlistConfig.Transport = mgwTransport
		} else {
			conf.MemberlistConfig.Transport = nt
		}
	}

	// Until Consul supports this fully, we disable automatic resolution.
	// When enabled, the Serf gossip may just turn off if we are the minority
	// node which is rather unexpected.
	conf.EnableNameConflictResolution = false

	if opts.WAN && s.config.ConnectMeshGatewayWANFederationEnabled {
		conf.MemberlistConfig.RequireNodeNames = true
		conf.MemberlistConfig.DisableTcpPingsForNode = func(nodeName string) bool {
			_, dc, err := wanfed.SplitNodeName(nodeName)
			if err != nil {
				return false // don't disable anything if we don't understand the node name
			}

			// If doing cross-dc we will be using TCP via the gateways so
			// there's no need for an extra TCP request.
			return s.config.Datacenter != dc
		}
	}

	if !s.config.DevMode {
		conf.SnapshotPath = filepath.Join(s.config.DataDir, opts.SnapshotPath)
	}
	if err := lib.EnsurePath(conf.SnapshotPath, false); err != nil {
		return nil, err
	}

	conf.ReconnectTimeoutOverride = libserf.NewReconnectOverride(s.logger)

	addSerfMetricsLabels(conf, opts.WAN, opts.Segment, s.config.AgentEnterpriseMeta().PartitionOrDefault(), "")

	addEnterpriseSerfTags(conf.Tags, s.config.AgentEnterpriseMeta())

	if s.config.OverrideInitialSerfTags != nil {
		s.config.OverrideInitialSerfTags(conf.Tags)
	}

	return conf, nil
}

// userEventName computes the name of a user event
func userEventName(name string) string {
	return userEventPrefix + name
}

// isUserEvent checks if a serf event is a user event
func isUserEvent(name string) bool {
	return strings.HasPrefix(name, userEventPrefix)
}

// rawUserEventName is used to get the raw user event name
func rawUserEventName(name string) string {
	return strings.TrimPrefix(name, userEventPrefix)
}

// lanEventHandler is used to handle events from the lan Serf cluster
func (s *Server) lanEventHandler() {
	for {
		select {
		case e := <-s.eventChLAN:
			switch e.EventType() {
			case serf.EventMemberJoin:
				s.lanNodeJoin(e.(serf.MemberEvent))
				s.localMemberEvent(e.(serf.MemberEvent))

			case serf.EventMemberLeave, serf.EventMemberFailed, serf.EventMemberReap:
				s.lanNodeFailed(e.(serf.MemberEvent))
				s.localMemberEvent(e.(serf.MemberEvent))

			case serf.EventUser:
				s.localEvent(e.(serf.UserEvent))
			case serf.EventMemberUpdate:
				s.lanNodeUpdate(e.(serf.MemberEvent))
				s.localMemberEvent(e.(serf.MemberEvent))
			case serf.EventQuery: // Ignore
			default:
				s.logger.Warn("Unhandled LAN Serf Event", "event", e)
			}

		case <-s.shutdownCh:
			return
		}
	}
}

// localMemberEvent is used to reconcile Serf events with the strongly
// consistent store if we are the current leader
func (s *Server) localMemberEvent(me serf.MemberEvent) {
	// Do nothing if we are not the leader
	if !s.IsLeader() {
		return
	}

	// Check if this is a reap event
	isReap := me.EventType() == serf.EventMemberReap

	// Queue the members for reconciliation
	for _, m := range me.Members {
		// Change the status if this is a reap event
		if isReap {
			m.Status = StatusReap
		}
		select {
		case s.reconcileCh <- m:
		default:
		}
	}
}

// localEvent is called when we receive an event on the local Serf
func (s *Server) localEvent(event serf.UserEvent) {
	// Handle only consul events
	if !strings.HasPrefix(event.Name, "consul:") {
		return
	}

	switch name := event.Name; {
	case name == newLeaderEvent:
		s.logger.Info("New leader elected", "payload", string(event.Payload))

		// Trigger the callback
		if s.config.ServerUp != nil {
			s.config.ServerUp()
		}
	case isUserEvent(name):
		event.Name = rawUserEventName(name)
		s.logger.Debug("User event", "event", event.Name)

		// Trigger the callback
		if s.config.UserEventHandler != nil {
			s.config.UserEventHandler(event)
		}
	default:
		if !s.handleEnterpriseUserEvents(event) {
			s.logger.Warn("Unhandled local event", "event", event)
		}
	}
}

// lanNodeJoin is used to handle join events on the LAN pool.
func (s *Server) lanNodeJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, serverMeta := metadata.IsConsulServer(m)
		if !ok || serverMeta.Segment != "" {
			continue
		}
		s.logger.Info("Adding LAN server", "server", serverMeta.String())

		// Update server lookup
		s.serverLookup.AddServer(serverMeta)
		s.router.AddServer(types.AreaLAN, serverMeta)

		// If we're still expecting to bootstrap, may need to handle this.
		if s.config.BootstrapExpect != 0 {
			s.maybeBootstrap()
		}

		// Kick the join flooders.
		s.FloodNotify()
	}
}

func (s *Server) lanNodeUpdate(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, serverMeta := metadata.IsConsulServer(m)
		if !ok || serverMeta.Segment != "" {
			continue
		}
		s.logger.Info("Updating LAN server", "server", serverMeta.String())

		// Update server lookup
		s.serverLookup.AddServer(serverMeta)
		s.router.AddServer(types.AreaLAN, serverMeta)
	}
}

// maybeBootstrap is used to handle bootstrapping when a new consul server joins.
func (s *Server) maybeBootstrap() {
	// Bootstrap can only be done if there are no committed logs, remove our
	// expectations of bootstrapping. This is slightly cheaper than the full
	// check that BootstrapCluster will do, so this is a good pre-filter.
	index, err := s.raftStore.LastIndex()
	if err != nil {
		s.logger.Error("Failed to read last raft index", "error", err)
		return
	}
	if index != 0 {
		s.logger.Info("Raft data found, disabling bootstrap mode")
		s.config.BootstrapExpect = 0
		return
	}

	if s.config.ReadReplica {
		s.logger.Info("Read replicas cannot bootstrap raft")
		return
	}

	// Scan for all the known servers.
	members := s.serfLAN.Members()
	var servers []metadata.Server
	voters := 0
	for _, member := range members {
		valid, p := metadata.IsConsulServer(member)
		if !valid {
			continue
		}
		if p.Datacenter != s.config.Datacenter {
			s.logger.Warn("Member has a conflicting datacenter, ignoring", "member", member)
			continue
		}
		if p.Expect != 0 && p.Expect != s.config.BootstrapExpect {
			s.logger.Error("Member has a conflicting expect value. All nodes should expect the same number.", "member", member)
			return
		}
		if p.Bootstrap {
			s.logger.Error("Member has bootstrap mode. Expect disabled.", "member", member)
			return
		}
		if !p.ReadReplica {
			voters++
		}
		servers = append(servers, *p)
	}

	// Skip if we haven't met the minimum expect count.
	if voters < s.config.BootstrapExpect {
		return
	}

	// Query each of the servers and make sure they report no Raft peers.
	for _, server := range servers {
		var peers []string

		// Retry with exponential backoff to get peer status from this server
		for attempt := uint(0); attempt < maxPeerRetries; attempt++ {
			if err := s.connPool.RPC(s.config.Datacenter, server.ShortName, server.Addr,
				"Status.Peers", &structs.DCSpecificRequest{Datacenter: s.config.Datacenter}, &peers); err != nil {
				nextRetry := (1 << attempt) * time.Second
				s.logger.Error("Failed to confirm peer status for server (will retry).",
					"server", server.Name,
					"retry_interval", nextRetry.String(),
					"error", err,
				)
				time.Sleep(nextRetry)
			} else {
				break
			}
		}

		// Found a node with some Raft peers, stop bootstrap since there's
		// evidence of an existing cluster. We should get folded in by the
		// existing servers if that's the case, so it's cleaner to sit as a
		// candidate with no peers so we don't cause spurious elections.
		// It's OK this is racy, because even with an initial bootstrap
		// as long as one peer runs bootstrap things will work, and if we
		// have multiple peers bootstrap in the same way, that's OK. We
		// just don't want a server added much later to do a live bootstrap
		// and interfere with the cluster. This isn't required for Raft's
		// correctness because no server in the existing cluster will vote
		// for this server, but it makes things much more stable.
		if len(peers) > 0 {
			s.logger.Info("Existing Raft peers reported by server, disabling bootstrap mode", "server", server.Name)
			s.config.BootstrapExpect = 0
			return
		}
	}

	// Attempt a live bootstrap!
	var configuration raft.Configuration
	var addrs []string

	for _, server := range servers {
		addr := server.Addr.String()
		addrs = append(addrs, addr)
		id := raft.ServerID(server.ID)

		suffrage := raft.Voter
		if server.ReadReplica {
			suffrage = raft.Nonvoter
		}
		peer := raft.Server{
			ID:       id,
			Address:  raft.ServerAddress(addr),
			Suffrage: suffrage,
		}
		configuration.Servers = append(configuration.Servers, peer)
	}
	s.logger.Info("Found expected number of peers, attempting bootstrap",
		"peers", strings.Join(addrs, ","),
	)
	future := s.raft.BootstrapCluster(configuration)
	if err := future.Error(); err != nil {
		s.logger.Error("Failed to bootstrap cluster", "error", err)
	}

	// Bootstrapping complete, or failed for some reason, don't enter this
	// again.
	s.config.BootstrapExpect = 0
}

// lanNodeFailed is used to handle fail events on the LAN pool.
func (s *Server) lanNodeFailed(me serf.MemberEvent) {
	for _, m := range me.Members {
		ok, serverMeta := metadata.IsConsulServer(m)
		if !ok || serverMeta.Segment != "" {
			continue
		}
		s.logger.Info("Removing LAN server", "server", serverMeta.String())

		// Update id to address map
		s.serverLookup.RemoveServer(serverMeta)
		s.router.RemoveServer(types.AreaLAN, serverMeta)
	}
}
