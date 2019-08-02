package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

type DiscoveryChain struct {
	srv *Server
}

func (c *DiscoveryChain) Get(args *structs.DiscoveryChainRequest, reply *structs.DiscoveryChainResponse) error {
	if done, err := c.srv.forward("DiscoveryChain.Get", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"discoverychain", "get"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.ServiceRead(args.Name) {
		return acl.ErrPermissionDenied
	}

	if args.Name == "" {
		return fmt.Errorf("Must provide service name")
	}

	evalDC := args.EvaluateInDatacenter
	if evalDC == "" {
		evalDC = c.srv.config.Datacenter
	}

	evalNS := args.EvaluateInNamespace
	if evalNS == "" {
		// TODO(namespaces) pull from something else?
		evalNS = "default"
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entries, err := state.ReadDiscoveryChainConfigEntries(ws, args.Name)
			if err != nil {
				return err
			}

			// Then we compile it into something useful.
			chain, err := discoverychain.Compile(discoverychain.CompileRequest{
				ServiceName:            args.Name,
				CurrentNamespace:       evalNS,
				CurrentDatacenter:      evalDC,
				OverrideMeshGateway:    args.OverrideMeshGateway,
				OverrideProtocol:       args.OverrideProtocol,
				OverrideConnectTimeout: args.OverrideConnectTimeout,
				Entries:                entries,
			})
			if err != nil {
				return err
			}

			reply.Index = index
			reply.Entries = entries.Flatten()
			reply.Chain = chain

			return nil
		})
}
