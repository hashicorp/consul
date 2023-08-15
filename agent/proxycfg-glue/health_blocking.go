// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxycfgglue

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/consul/watch"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/structs/aclfilter"
)

// ServerHealthBlocking exists due to a bug with the streaming backend and its interaction with ACLs.
// Whenever an exported-services config entry is modified, this is effectively an ACL change.
// Assume the following situation:
//   - no services are exported
//   - an upstream watch to service X is spawned
//   - the streaming backend filters out data for service X (because it's not exported yet)
//   - service X is finally exported
//
// In this situation, the streaming backend does not trigger a refresh of its data.
// This means that any events that were supposed to have been received prior to the export are NOT backfilled,
// and the watches never see service X spawning.
//
// We currently have decided to not trigger a stream refresh in this situation due to the potential for a
// thundering herd effect (touching exports would cause a re-fetch of all watches for that partition, potentially).
// Therefore, this local blocking-query approach exists for agentless.
//
// It's also worth noting that the streaming subscription is currently bypassed most of the time with agentful,
// because proxycfg has a `req.Source.Node != ""` which prevents the `streamingEnabled` check from passing.
// This means that while agents should technically have this same issue, they don't experience it with mesh health
// watches.
func ServerHealthBlocking(deps ServerDataSourceDeps, remoteSource proxycfg.Health, state *state.Store) *serverHealthBlocking {
	return &serverHealthBlocking{deps, remoteSource, state, 5 * time.Minute}
}

type serverHealthBlocking struct {
	deps         ServerDataSourceDeps
	remoteSource proxycfg.Health
	state        *state.Store
	watchTimeout time.Duration
}

// Notify is mostly a copy of the function in `agent/consul/health_endpoint.go` with a few minor tweaks.
// Most notably, some query features unnecessary for mesh have been stripped out.
func (h *serverHealthBlocking) Notify(ctx context.Context, args *structs.ServiceSpecificRequest, correlationID string, ch chan<- proxycfg.UpdateEvent) error {
	if args.Datacenter != h.deps.Datacenter {
		return h.remoteSource.Notify(ctx, args, correlationID, ch)
	}

	// Verify the arguments
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide service name")
	}
	if args.EnterpriseMeta.PartitionOrDefault() == acl.WildcardName {
		return fmt.Errorf("Wildcards are not allowed in the partition field")
	}

	// Determine the function we'll call
	var f func(memdb.WatchSet, *state.Store, *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error)
	switch {
	case args.Connect:
		f = serviceNodesConnect
	case args.Ingress:
		f = serviceNodesIngress
	default:
		f = serviceNodesDefault
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, structs.CheckServiceNode{})
	if err != nil {
		return err
	}

	var hadResults bool = false
	return watch.ServerLocalNotify(ctx, correlationID, h.deps.GetStore,
		func(ws memdb.WatchSet, store Store) (uint64, *structs.IndexedCheckServiceNodes, error) {
			// This is necessary so that service export changes are eventually picked up, since
			// they won't trigger the watch themselves.
			timeoutCh := make(chan struct{})
			time.AfterFunc(h.watchTimeout, func() {
				close(timeoutCh)
			})
			ws.Add(timeoutCh)

			authzContext := acl.AuthorizerContext{
				Peer: args.PeerName,
			}
			authz, err := h.deps.ACLResolver.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
			if err != nil {
				return 0, nil, err
			}
			// If we're doing a connect or ingress query, we need read access to the service
			// we're trying to find proxies for, so check that.
			if args.Connect || args.Ingress {
				if authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
					// If access was somehow revoked (via token deletion or unexporting), then we clear the
					// last-known results before triggering an error. This way, the proxies will actually update
					// their data, rather than holding onto the last-known list of healthy nodes indefinitely.
					if hadResults {
						hadResults = false
						return 0, &structs.IndexedCheckServiceNodes{}, watch.ErrorACLResetData
					}
					return 0, nil, acl.ErrPermissionDenied
				}
			}

			var thisReply structs.IndexedCheckServiceNodes
			thisReply.Index, thisReply.Nodes, err = f(ws, h.state, args)
			if err != nil {
				return 0, nil, err
			}

			raw, err := filter.Execute(thisReply.Nodes)
			if err != nil {
				return 0, nil, err
			}
			thisReply.Nodes = raw.(structs.CheckServiceNodes)

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := h.filterACL(&authzContext, args.Token, &thisReply); err != nil {
				return 0, nil, err
			}

			hadResults = true
			return thisReply.Index, &thisReply, nil
		},
		dispatchBlockingQueryUpdate[*structs.IndexedCheckServiceNodes](ch),
	)
}

func (h *serverHealthBlocking) filterACL(authz *acl.AuthorizerContext, token string, subj *structs.IndexedCheckServiceNodes) error {
	// Get the ACL from the token
	var entMeta acl.EnterpriseMeta
	authorizer, err := h.deps.ACLResolver.ResolveTokenAndDefaultMeta(token, &entMeta, authz)
	if err != nil {
		return err
	}
	aclfilter.New(authorizer, h.deps.Logger).Filter(subj)
	return nil
}

func serviceNodesConnect(ws memdb.WatchSet, s *state.Store, args *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error) {
	return s.CheckConnectServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta, args.PeerName)
}

func serviceNodesIngress(ws memdb.WatchSet, s *state.Store, args *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error) {
	return s.CheckIngressServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta)
}

func serviceNodesDefault(ws memdb.WatchSet, s *state.Store, args *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error) {
	return s.CheckServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta, args.PeerName)
}
