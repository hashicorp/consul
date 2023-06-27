// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

// Usage returns counts for service usage within catalog.
func (op *Operator) Usage(args *structs.OperatorUsageRequest, reply *structs.Usage) error {
	reply.Usage = make(map[string]structs.ServiceUsage)

	if args.Global {
		remoteDCs := op.srv.router.GetDatacenters()
		for _, dc := range remoteDCs {
			remoteArgs := &structs.OperatorUsageRequest{
				DCSpecificRequest: structs.DCSpecificRequest{
					Datacenter: dc,
					QueryOptions: structs.QueryOptions{
						Token: args.Token,
					},
				},
			}
			var resp structs.Usage
			if _, err := op.srv.ForwardRPC("Operator.Usage", remoteArgs, &resp); err != nil {
				op.logger.Warn("error forwarding usage request to remote datacenter", "datacenter", dc, "error", err)
			}
			if usage, ok := resp.Usage[dc]; ok {
				reply.Usage[dc] = usage
			}
		}
	}

	var authzContext acl.AuthorizerContext
	authz, err := op.srv.ResolveTokenAndDefaultMeta(args.Token, structs.DefaultEnterpriseMetaInDefaultPartition(), &authzContext)
	if err != nil {
		return err
	}
	err = authz.ToAllowAuthorizer().OperatorReadAllowed(&authzContext)
	if err != nil {
		return err
	}

	if err = op.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return op.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			// Get service usage.
			index, serviceUsage, err := state.ServiceUsage(ws)
			if err != nil {
				return err
			}

			reply.QueryMeta.Index, reply.Usage[op.srv.config.Datacenter] = index, serviceUsage
			return nil
		})
}
