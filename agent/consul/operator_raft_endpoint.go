// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"fmt"
	"net"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
)

// RaftGetConfiguration is used to retrieve the current Raft configuration.
func (op *Operator) RaftGetConfiguration(args *structs.DCSpecificRequest, reply *structs.RaftConfigurationResponse) error {
	if done, err := op.srv.ForwardRPC("Operator.RaftGetConfiguration", args, reply); done {
		return err
	}

	// This action requires operator read access.
	authz, err := op.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorReadAllowed(nil); err != nil {
		return err
	}

	// We can't fetch the leader and the configuration atomically with
	// the current Raft API.
	future := op.srv.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	// Index the Consul information about the servers.
	serverMap := make(map[raft.ServerAddress]serf.Member)
	for _, member := range op.srv.serfLAN.Members() {
		valid, parts := metadata.IsConsulServer(member)
		if !valid {
			continue
		}

		addr := (&net.TCPAddr{IP: member.Addr, Port: parts.Port}).String()
		serverMap[raft.ServerAddress(addr)] = member
	}

	serverIDLastIndexMap := make(map[raft.ServerID]uint64)

	for _, serverState := range op.srv.autopilot.GetState().Servers {
		serverIDLastIndexMap[serverState.Server.ID] = serverState.Stats.LastIndex
	}

	// Fill out the reply.
	leader := op.srv.raft.Leader()
	reply.Index = future.Index()
	for _, server := range future.Configuration().Servers {
		node := "(unknown)"
		raftProtocolVersion := "unknown"
		if member, ok := serverMap[server.Address]; ok {
			node = member.Name
			raftProtocolVersion = member.Tags["raft_vsn"]
		}

		entry := &structs.RaftServer{
			ID:              server.ID,
			Node:            node,
			Address:         server.Address,
			Leader:          server.Address == leader,
			Voter:           server.Suffrage == raft.Voter,
			ProtocolVersion: raftProtocolVersion,
			LastIndex:       serverIDLastIndexMap[server.ID],
		}
		reply.Servers = append(reply.Servers, entry)
	}
	return nil
}

// RaftRemovePeerByAddress is used to kick a stale peer (one that it in the Raft
// quorum but no longer known to Serf or the catalog) by address in the form of
// "IP:port". The reply argument is not used, but it required to fulfill the RPC
// interface.
func (op *Operator) RaftRemovePeerByAddress(args *structs.RaftRemovePeerRequest, reply *struct{}) error {
	if done, err := op.srv.ForwardRPC("Operator.RaftRemovePeerByAddress", args, reply); done {
		return err
	}

	// This is a super dangerous operation that requires operator write
	// access.
	authz, err := op.srv.ACLResolver.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(authz.Identity()); err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied address isn't among the peers since it's
	// likely they screwed up.
	{
		future := op.srv.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return err
		}
		for _, s := range future.Configuration().Servers {
			if s.Address == args.Address {
				args.ID = s.ID
				goto REMOVE
			}
		}
		return fmt.Errorf("address %q was not found in the Raft configuration",
			args.Address)
	}

REMOVE:
	if err := op.srv.autopilot.RemoveServer(args.ID); err != nil {
		op.logger.Warn("Failed to remove Raft server",
			"address", args.Address,
			"error", err,
		)
		return err
	}

	op.logger.Warn("Removed Raft server", "address", args.Address)
	return nil
}

// RaftRemovePeerByID is used to kick a stale peer (one that is in the Raft
// quorum but no longer known to Serf or the catalog) by address in the form of
// "IP:port". The reply argument is not used, but is required to fulfill the RPC
// interface.
func (op *Operator) RaftRemovePeerByID(args *structs.RaftRemovePeerRequest, reply *struct{}) error {
	if done, err := op.srv.ForwardRPC("Operator.RaftRemovePeerByID", args, reply); done {
		return err
	}

	// This is a super dangerous operation that requires operator write
	// access.
	authz, err := op.srv.ACLResolver.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := op.srv.validateEnterpriseToken(authz.Identity()); err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	// Since this is an operation designed for humans to use, we will return
	// an error if the supplied id isn't among the peers since it's
	// likely they screwed up.
	{
		future := op.srv.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return err
		}
		for _, s := range future.Configuration().Servers {
			if s.ID == args.ID {
				args.Address = s.Address
				goto REMOVE
			}
		}
		return fmt.Errorf("id %q was not found in the Raft configuration",
			args.ID)
	}

REMOVE:
	// The Raft library itself will prevent various forms of foot-shooting,
	// like making a configuration with no voters. Some consideration was
	// given here to adding more checks, but it was decided to make this as
	// low-level and direct as possible. We've got ACL coverage to lock this
	// down, and if you are an operator, it's assumed you know what you are
	// doing if you are calling this. If you remove a peer that's known to
	// Serf, for example, it will come back when the leader does a reconcile
	// pass.
	if err := op.srv.autopilot.RemoveServer(args.ID); err != nil {
		op.logger.Warn("Failed to remove Raft peer with id",
			"peer_id", args.ID,
			"error", err,
		)
		return err
	}

	op.logger.Warn("Removed Raft peer with id", "peer_id", args.ID)
	return nil
}
