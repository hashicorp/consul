package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

type DiscoveryChain struct {
	srv *Server
}

func (c *DiscoveryChain) Get(args *structs.DiscoveryChainRequest, reply *structs.DiscoveryChainResponse) error {
	// Exit early if Connect hasn't been enabled.
	if !c.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := c.srv.ForwardRPC("DiscoveryChain.Get", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"discovery_chain", "get"}, time.Now())

	// Fetch the ACL token, if any.
	entMeta := args.GetEnterpriseMeta()
	var authzContext acl.AuthorizerContext
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, entMeta, &authzContext)
	if err != nil {
		return err
	}
	if authz.ServiceRead(args.Name, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	if args.Name == "" {
		return fmt.Errorf("Must provide service name")
	}

	evalDC := args.EvaluateInDatacenter
	if evalDC == "" {
		evalDC = c.srv.config.Datacenter
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			req := discoverychain.CompileRequest{
				ServiceName:            args.Name,
				EvaluateInNamespace:    entMeta.NamespaceOrDefault(),
				EvaluateInPartition:    entMeta.PartitionOrDefault(),
				EvaluateInDatacenter:   evalDC,
				OverrideMeshGateway:    args.OverrideMeshGateway,
				OverrideProtocol:       args.OverrideProtocol,
				OverrideConnectTimeout: args.OverrideConnectTimeout,
			}
			index, chain, err := state.ServiceDiscoveryChain(ws, args.Name, entMeta, req)
			if err != nil {
				return err
			}

			reply.Index = index
			reply.Chain = chain

			return nil
		})
}
