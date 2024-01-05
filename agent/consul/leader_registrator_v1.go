// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/fsm"
	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

var _ ConsulRegistrator = (*V1ConsulRegistrator)(nil)

type V1ConsulRegistrator struct {
	Datacenter string
	FSM        *fsm.FSM
	Logger     hclog.Logger
	NodeName   string

	RaftApplyFunc func(t structs.MessageType, msg any) (any, error)
}

// HandleAliveMember is used to ensure the node
// is registered, with a passing health check.
func (r V1ConsulRegistrator) HandleAliveMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta, joinServer func(m serf.Member, parts *metadata.Server) error) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Register consul service if a server
	var service *structs.NodeService
	if valid, parts := metadata.IsConsulServer(member); valid {
		service = &structs.NodeService{
			ID:      structs.ConsulServiceID,
			Service: structs.ConsulServiceName,
			Port:    parts.Port,
			Weights: &structs.Weights{
				Passing: 1,
				Warning: 1,
			},
			EnterpriseMeta: *nodeEntMeta,
			Meta: map[string]string{
				// DEPRECATED - remove nonvoter in favor of read_replica in a future version of consul
				"non_voter":             strconv.FormatBool(member.Tags["nonvoter"] == "1"),
				"read_replica":          strconv.FormatBool(member.Tags["read_replica"] == "1"),
				"raft_version":          strconv.Itoa(parts.RaftVersion),
				"serf_protocol_current": strconv.FormatUint(uint64(member.ProtocolCur), 10),
				"serf_protocol_min":     strconv.FormatUint(uint64(member.ProtocolMin), 10),
				"serf_protocol_max":     strconv.FormatUint(uint64(member.ProtocolMax), 10),
				"version":               parts.Build.String(),
			},
		}

		if parts.ExternalGRPCPort > 0 {
			service.Meta["grpc_port"] = strconv.Itoa(parts.ExternalGRPCPort)
		}
		if parts.ExternalGRPCTLSPort > 0 {
			service.Meta["grpc_tls_port"] = strconv.Itoa(parts.ExternalGRPCTLSPort)
		}

		// Attempt to join the consul server
		if err := joinServer(member, parts); err != nil {
			return err
		}
	}

	// Check if the node exists
	state := r.FSM.State()
	_, node, err := state.GetNode(member.Name, nodeEntMeta, structs.DefaultPeerKeyword)
	if err != nil {
		return err
	}
	if node != nil && node.Address == member.Addr.String() {
		// Check if the associated service is available
		if service != nil {
			match := false
			_, services, err := state.NodeServices(nil, member.Name, nodeEntMeta, structs.DefaultPeerKeyword)
			if err != nil {
				return err
			}
			if services != nil {
				for id, serv := range services.Services {
					if id == service.ID {
						// If metadata are different, be sure to update it
						match = reflect.DeepEqual(serv.Meta, service.Meta)
					}
				}
			}
			if !match {
				goto AFTER_CHECK
			}
		}

		// Check if the serfCheck is in the passing state
		_, checks, err := state.NodeChecks(nil, member.Name, nodeEntMeta, structs.DefaultPeerKeyword)
		if err != nil {
			return err
		}
		for _, check := range checks {
			if check.CheckID == structs.SerfCheckID && check.Status == api.HealthPassing {
				return nil
			}
		}
	}
AFTER_CHECK:
	r.Logger.Info("member joined, marking health alive",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

	// Get consul version from serf member
	// add this as node meta in catalog register request
	buildVersion, err := metadata.Build(&member)
	if err != nil {
		return err
	}

	// Register with the catalog.
	req := structs.RegisterRequest{
		Datacenter: r.Datacenter,
		Node:       member.Name,
		ID:         types.NodeID(member.Tags["id"]),
		Address:    member.Addr.String(),
		Service:    service,
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: structs.SerfCheckID,
			Name:    structs.SerfCheckName,
			Status:  api.HealthPassing,
			Output:  structs.SerfCheckAliveOutput,
		},
		EnterpriseMeta: *nodeEntMeta,
		NodeMeta: map[string]string{
			structs.MetaConsulVersion: buildVersion.String(),
		},
	}
	if node != nil {
		req.TaggedAddresses = node.TaggedAddresses
		req.NodeMeta = node.Meta
	}

	_, err = r.RaftApplyFunc(structs.RegisterRequestType, &req)
	return err
}

// HandleFailedMember is used to mark the node's status
// as being critical, along with all checks as unknown.
func (r V1ConsulRegistrator) HandleFailedMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Check if the node exists
	state := r.FSM.State()
	_, node, err := state.GetNode(member.Name, nodeEntMeta, structs.DefaultPeerKeyword)
	if err != nil {
		return err
	}

	if node == nil {
		r.Logger.Info("ignoring failed event for member because it does not exist in the catalog",
			"member", member.Name,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}

	if node.Address == member.Addr.String() {
		// Check if the serfCheck is in the critical state
		_, checks, err := state.NodeChecks(nil, member.Name, nodeEntMeta, structs.DefaultPeerKeyword)
		if err != nil {
			return err
		}
		for _, check := range checks {
			if check.CheckID == structs.SerfCheckID && check.Status == api.HealthCritical {
				return nil
			}
		}
	}
	r.Logger.Info("member failed, marking health critical",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
	)

	// Register with the catalog
	req := structs.RegisterRequest{
		Datacenter:     r.Datacenter,
		Node:           member.Name,
		EnterpriseMeta: *nodeEntMeta,
		ID:             types.NodeID(member.Tags["id"]),
		Address:        member.Addr.String(),
		Check: &structs.HealthCheck{
			Node:    member.Name,
			CheckID: structs.SerfCheckID,
			Name:    structs.SerfCheckName,
			Status:  api.HealthCritical,
			Output:  structs.SerfCheckFailedOutput,
		},

		// If there's existing information about the node, do not
		// clobber it.
		SkipNodeUpdate: true,
	}
	_, err = r.RaftApplyFunc(structs.RegisterRequestType, &req)
	return err
}

// HandleLeftMember is used to handle members that gracefully
// left. They are deregistered if necessary.
func (r V1ConsulRegistrator) HandleLeftMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta, removeServerFunc func(m serf.Member) error) error {
	return r.handleDeregisterMember("left", member, nodeEntMeta, removeServerFunc)
}

// HandleReapMember is used to handle members that have been
// reaped after a prolonged failure. They are deregistered.
func (r V1ConsulRegistrator) HandleReapMember(member serf.Member, nodeEntMeta *acl.EnterpriseMeta, removeServerFunc func(m serf.Member) error) error {
	return r.handleDeregisterMember("reaped", member, nodeEntMeta, removeServerFunc)
}

// handleDeregisterMember is used to deregister a member of a given reason
func (r V1ConsulRegistrator) handleDeregisterMember(reason string, member serf.Member, nodeEntMeta *acl.EnterpriseMeta, removeServerFunc func(m serf.Member) error) error {
	if nodeEntMeta == nil {
		nodeEntMeta = structs.NodeEnterpriseMetaInDefaultPartition()
	}

	// Do not deregister ourself. This can only happen if the current leader
	// is leaving. Instead, we should allow a follower to take-over and
	// deregister us later.
	//
	// TODO(partitions): check partitions here too? server names should be unique in general though
	if strings.EqualFold(member.Name, r.NodeName) {
		r.Logger.Warn("deregistering self should be done by follower",
			"name", r.NodeName,
			"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		)
		return nil
	}

	// Remove from Raft peers if this was a server
	if valid, _ := metadata.IsConsulServer(member); valid {
		if err := removeServerFunc(member); err != nil {
			return err
		}
	}

	// Check if the node does not exist
	state := r.FSM.State()
	_, node, err := state.GetNode(member.Name, nodeEntMeta, structs.DefaultPeerKeyword)
	if err != nil {
		return err
	}
	if node == nil {
		return nil
	}

	// Deregister the node
	r.Logger.Info("deregistering member",
		"member", member.Name,
		"partition", getSerfMemberEnterpriseMeta(member).PartitionOrDefault(),
		"reason", reason,
	)
	req := structs.DeregisterRequest{
		Datacenter:     r.Datacenter,
		Node:           member.Name,
		EnterpriseMeta: *nodeEntMeta,
	}
	_, err = r.RaftApplyFunc(structs.DeregisterRequestType, &req)
	return err
}
