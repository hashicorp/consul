package consul

import (
	"errors"
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/discoverychain"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

type DiscoveryChain struct {
	srv *Server
}

func (c *DiscoveryChain) Get(args *structs.DiscoveryChainRequest, reply *structs.DiscoveryChainResponse) error {
	// Exit early if Connect hasn't been enabled.
	if !c.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := c.srv.forward("DiscoveryChain.Get", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"discovery_chain", "get"}, time.Now())

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

			_, config, err := state.CAConfig(ws)
			if err != nil {
				return err
			} else if config == nil {
				return errors.New("no cluster ca config setup")
			}

			// Build TrustDomain based on the ClusterID stored.
			signingID := connect.SpiffeIDSigningForCluster(config)
			if signingID == nil {
				// If CA is bootstrapped at all then this should never happen but be
				// defensive.
				return errors.New("no cluster trust domain setup")
			}
			currentTrustDomain := signingID.Host()

			// Then we compile it into something useful.
			chain, err := discoverychain.Compile(discoverychain.CompileRequest{
				ServiceName:            args.Name,
				EvaluateInNamespace:    evalNS,
				EvaluateInDatacenter:   evalDC,
				EvaluateInTrustDomain:  currentTrustDomain,
				UseInDatacenter:        c.srv.config.Datacenter,
				OverrideMeshGateway:    args.OverrideMeshGateway,
				OverrideProtocol:       args.OverrideProtocol,
				OverrideConnectTimeout: args.OverrideConnectTimeout,
				Entries:                entries,
			})
			if err != nil {
				return err
			}

			reply.Index = index
			reply.Chain = chain

			return nil
		})
}
