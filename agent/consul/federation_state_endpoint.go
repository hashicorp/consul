// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

var FederationStateSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"federation_state", "apply"},
		Help: "",
	},
	{
		Name: []string{"federation_state", "get"},
		Help: "",
	},
	{
		Name: []string{"federation_state", "list"},
		Help: "",
	},
	{
		Name: []string{"federation_state", "list_mesh_gateways"},
		Help: "",
	},
}

var (
	errFederationStatesNotEnabled = errors.New("Federation states are currently disabled until all servers in the datacenter support the feature")
)

// FederationState endpoint is used to manipulate federation states from all
// datacenters.
type FederationState struct {
	srv *Server
}

func (c *FederationState) Apply(args *structs.FederationStateRequest, reply *bool) error {
	// Ensure that all federation state writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.ForwardRPC("FederationState.Apply", args, reply); done {
		return err
	}

	if !c.srv.DatacenterSupportsFederationStates() {
		return errFederationStatesNotEnabled
	}

	defer metrics.MeasureSince([]string{"federation_state", "apply"}, time.Now())

	// Fetch the ACL token, if any.
	authz, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorWriteAllowed(nil); err != nil {
		return err
	}

	if args.State == nil || args.State.Datacenter == "" {
		return fmt.Errorf("invalid request: missing federation state datacenter")
	}

	switch args.Op {
	case structs.FederationStateUpsert:
		if args.State.UpdatedAt.IsZero() {
			args.State.UpdatedAt = time.Now().UTC()
		}
	case structs.FederationStateDelete:
		// No validation required.
	default:
		return fmt.Errorf("Invalid federation state operation: %v", args.Op)
	}

	resp, err := c.srv.raftApply(structs.FederationStateRequestType, args)
	if err != nil {
		return err
	}
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}

	return nil
}

func (c *FederationState) Get(args *structs.FederationStateQuery, reply *structs.FederationStateResponse) error {
	if done, err := c.srv.ForwardRPC("FederationState.Get", args, reply); done {
		return err
	}

	if !c.srv.DatacenterSupportsFederationStates() {
		return errFederationStatesNotEnabled
	}

	defer metrics.MeasureSince([]string{"federation_state", "get"}, time.Now())

	// Fetch the ACL token, if any.
	authz, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorReadAllowed(nil); err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, fedState, err := state.FederationStateGet(ws, args.Datacenter)
			if err != nil {
				return err
			}

			reply.Index, reply.State = index, fedState
			if fedState == nil {
				return errNotFound
			}
			return nil
		})
}

// List is the endpoint meant to be used by consul servers performing
// replication.
func (c *FederationState) List(args *structs.DCSpecificRequest, reply *structs.IndexedFederationStates) error {
	if done, err := c.srv.ForwardRPC("FederationState.List", args, reply); done {
		return err
	}

	if !c.srv.DatacenterSupportsFederationStates() {
		return errFederationStatesNotEnabled
	}

	defer metrics.MeasureSince([]string{"federation_state", "list"}, time.Now())

	// Fetch the ACL token, if any.
	authz, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := authz.ToAllowAuthorizer().OperatorReadAllowed(nil); err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, fedStates, err := state.FederationStateList(ws)
			if err != nil {
				return err
			}

			if len(fedStates) == 0 {
				fedStates = []*structs.FederationState{}
			}

			reply.Index = index
			reply.States = fedStates

			return nil
		})
}

// ListMeshGateways is the endpoint meant to be used by proxies only interested
// in the discovery info for dialing mesh gateways. Analogous to catalog
// endpoints.
func (c *FederationState) ListMeshGateways(args *structs.DCSpecificRequest, reply *structs.DatacenterIndexedCheckServiceNodes) error {
	if done, err := c.srv.ForwardRPC("FederationState.ListMeshGateways", args, reply); done {
		return err
	}

	if !c.srv.DatacenterSupportsFederationStates() {
		return errFederationStatesNotEnabled
	}

	defer metrics.MeasureSince([]string{"federation_state", "list_mesh_gateways"}, time.Now())

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, fedStates, err := state.FederationStateList(ws)
			if err != nil {
				return err
			}

			dump := make(map[string]structs.CheckServiceNodes)

			for i := range fedStates {
				fedState := fedStates[i]
				csn := fedState.MeshGateways
				if len(csn) > 0 {
					// We shallow clone this slice so that the filterACL doesn't
					// end up manipulating the slice in memdb.
					dump[fedState.Datacenter] = csn.ShallowClone()
				}
			}

			reply.Index, reply.DatacenterNodes = index, dump
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return nil
		})
}
