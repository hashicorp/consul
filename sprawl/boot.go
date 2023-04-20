package sprawl

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/consul-topology/sprawl/internal/build"
	"github.com/hashicorp/consul-topology/sprawl/internal/secrets"
	"github.com/hashicorp/consul-topology/sprawl/internal/tfgen"
	"github.com/hashicorp/consul-topology/topology"
	"github.com/hashicorp/consul-topology/util"
)

const (
	sharedBootstrapToken = "root"
	// sharedBootstrapToken = "ec59aa56-1996-4ff1-911a-f5d782552a13"

	sharedAgentRecoveryToken = "22082b05-05c9-4a0a-b3da-b9685ac1d688"
)

func (s *Sprawl) launch() error {
	return s.launchType(true)
}
func (s *Sprawl) relaunch() error {
	return s.launchType(false)
}
func (s *Sprawl) launchType(firstTime bool) (launchErr error) {
	if err := build.DockerImages(s.logger, s.runner, s.topology); err != nil {
		return fmt.Errorf("build.DockerImages: %w", err)
	}

	if firstTime {
		// Initialize secrets the easy way for now (same in all clusters).
		gossipKey, err := newGossipKey()
		if err != nil {
			return fmt.Errorf("newGossipKey: %w", err)
		}
		for _, cluster := range s.topology.Clusters {
			s.secrets.SaveGeneric(cluster.Name, secrets.BootstrapToken, sharedBootstrapToken)
			s.secrets.SaveGeneric(cluster.Name, secrets.AgentRecovery, sharedAgentRecoveryToken)
			s.secrets.SaveGeneric(cluster.Name, secrets.GossipKey, gossipKey)

			// Give servers a copy of the bootstrap token for use as their agent tokens
			// to avoid complicating the chicken/egg situation for startup.
			for _, node := range cluster.Nodes {
				if node.IsServer() { // include disabled
					s.secrets.SaveAgentToken(cluster.Name, node.ID(), sharedBootstrapToken)
				}
			}
		}
	}

	var cleanupFuncs []func()
	defer func() {
		for i := len(cleanupFuncs) - 1; i >= 0; i-- {
			cleanupFuncs[i]()
		}
	}()

	var relaunchDeferredFunc []func() error

	if firstTime {
		var err error
		s.generator, err = tfgen.NewGenerator(
			s.logger.Named("tfgen"),
			s.runner,
			s.topology,
			&s.secrets,
			s.workdir,
			s.license,
		)
		if err != nil {
			return err
		}
	} else {
		s.generator.SetTopology(s.topology)
	}
	cleanupFuncs = append(cleanupFuncs, func() {
		// Log the error before the cleanup so you don't have to wait to see
		// the cause.
		if launchErr != nil {
			s.logger.Error("fatal error during launch", "error", launchErr)
		}

		_ = s.generator.DestroyAllQuietly()
	})

	if firstTime {
		// The networking phase is special. We have to pick a random subnet and
		// hope. Once we have this established once it is immutable for future
		// runs.
		if err := s.initNetworking(); err != nil {
			return fmt.Errorf("initNetworking: %w", err)
		}
	}

	if err := s.assignIPAddresses(); err != nil {
		return fmt.Errorf("assignIPAddresses: %w", err)
	}

	// The previous terraform run should have made the special volume for us.
	if err := s.initTLS(); err != nil {
		return fmt.Errorf("initTLS: %w", err)
	}

	if err := s.initConsulServers(firstTime); err != nil {
		return fmt.Errorf("initConsulServers: %w", err)
	}

	if firstTime {
		if err := s.generator.Generate(tfgen.StepAgents); err != nil {
			return fmt.Errorf("generator[agents]: %w", err)
		}
		for _, cluster := range s.topology.Clusters {
			if err := s.waitForClientAntiEntropyOnce(cluster); err != nil {
				return fmt.Errorf("waitForClientAntiEntropyOnce[%s]: %w", cluster.Name, err)
			}
		}
	} else {
		relaunchDeferredFunc = append(relaunchDeferredFunc, func() error {
			for _, cluster := range s.topology.Clusters {
				if err := s.waitForClientAntiEntropyOnce(cluster); err != nil {
					return fmt.Errorf("waitForClientAntiEntropyOnce[%s]: %w", cluster.Name, err)
				}
			}
			return nil
		})
	}

	// Ideally we start services WITH a token initially, so we pre-create them
	// before running terraform for them.
	if err := s.createAllServiceTokens(); err != nil {
		return fmt.Errorf("createAllServiceTokens: %w", err)
	}

	if err := s.registerAllServicesForDataplaneInstances(); err != nil {
		return fmt.Errorf("registerAllServicesForDataplaneInstances: %w", err)
	}

	if firstTime {
		// We can do this ahead, because we've incrementally run terraform as
		// we went.
		if err := s.registerAllServicesToAgents(); err != nil {
			return fmt.Errorf("registerAllServicesToAgents: %w", err)
		}
		// NOTE: start services WITH token initially
		if err := s.generator.Generate(tfgen.StepServices); err != nil {
			return fmt.Errorf("generator[services]: %w", err)
		}
	} else {
		relaunchDeferredFunc = append(relaunchDeferredFunc, func() error {
			if err := s.registerAllServicesToAgents(); err != nil {
				return fmt.Errorf("registerAllServicesToAgents: %w", err)
			}
			return nil
		})
	}

	if !firstTime {
		// We save all of the terraform to the end. Some of the containers will
		// be a little broken until we can do stuff like register services to
		// new agents, which we cannot do until they come up.
		if err := s.generator.Generate(tfgen.StepRelaunch); err != nil {
			return fmt.Errorf("generator[relaunch]: %w", err)
		}

		for _, fn := range relaunchDeferredFunc {
			if err := fn(); err != nil {
				return err
			}
		}
	}

	if firstTime {
		if err := s.initPeerings(); err != nil {
			return fmt.Errorf("initPeerings: %w", err)
		}
	}

	// TODO: verify peering

	cleanupFuncs = nil // reset

	if firstTime {
		s.generator.MarkLaunched()
	}

	return nil
}

func (s *Sprawl) Stop() error {
	var merr error
	if s.generator != nil {
		if err := s.generator.DestroyAllQuietly(); err != nil {
			merr = multierror.Append(merr, err)
		}
	}
	return merr
}

func (s *Sprawl) initNetworking() error {
	// TODO: loop this on a very specific error
	if err := s.generator.Generate(tfgen.StepNetworks); err != nil {
		return fmt.Errorf("generator[networks]: %w", err)
	}

	return nil
}

func (s *Sprawl) assignIPAddresses() error {
	// assign ips now that we have network ips known to us
	for _, net := range s.topology.Networks {
		if len(net.IPPool) == 0 {
			return fmt.Errorf("network %q does not have any ip assignments", net.Name)
		}
	}
	for _, cluster := range s.topology.Clusters {
		for _, node := range cluster.Nodes {
			for _, addr := range node.Addresses {
				net, ok := s.topology.Networks[addr.Network]
				if !ok {
					return fmt.Errorf("unknown network %q", addr.Network)
				}
				addr.IPAddress = net.IPByIndex(node.Index)
			}
		}
	}

	return nil
}

func (s *Sprawl) initConsulServers(firstTime bool) error {
	if firstTime {
		if err := s.generator.Generate(tfgen.StepServers); err != nil {
			return fmt.Errorf("generator[servers]: %w", err)
		}

		s.logger.Info("ALL", "t", jd(s.topology)) // TODO

		// Create token-less api clients first.
		for _, cluster := range s.topology.Clusters {
			node := cluster.FirstServer()

			var err error
			s.clients[cluster.Name], err = util.ProxyAPIClient(
				node.LocalSquidPort(),
				node.LocalAddress(),
				8500,
				"", /*no token yet*/
			)
			if err != nil {
				return fmt.Errorf("error creating initial bootstrap client for cluster=%s: %w", cluster.Name, err)
			}
		}
	}

	if firstTime {
		// Join the servers together.
		for _, cluster := range s.topology.Clusters {
			if err := s.rejoinServers(cluster, firstTime); err != nil {
				return fmt.Errorf("rejoinServers[%s]: %w", cluster.Name, err)
			}
			s.waitForLeader(cluster)
		}
	}

	for _, cluster := range s.topology.Clusters {
		var err error
		if firstTime {
			err = s.bootstrapACLs(cluster.Name)
			if err != nil {
				return fmt.Errorf("bootstrap[%s]: %w", cluster.Name, err)
			}
		}

		mgmtToken := s.secrets.ReadGeneric(cluster.Name, secrets.BootstrapToken)

		// Reconfigure the clients to use a management token.
		node := cluster.FirstServer()
		s.clients[cluster.Name], err = util.ProxyAPIClient(
			node.LocalSquidPort(),
			node.LocalAddress(),
			8500,
			mgmtToken,
		)
		if err != nil {
			return fmt.Errorf("error creating final client for cluster=%s: %v", cluster.Name, err)
		}

		if !firstTime {
			s.waitForLeader(cluster)
		}

		// For some reason the grpc resolver stuff for partitions takes some
		// time to get ready.
		s.waitForLocalWrites(cluster, mgmtToken)

		// Create tenancies so that the ACL tokens and clients have somewhere to go.
		if cluster.Enterprise {
			if err := s.initTenancies(cluster); err != nil {
				return fmt.Errorf("initTenancies[%s]: %w", cluster.Name, err)
			}
		}

		if firstTime {
			if err := s.populateInitialConfigEntries(cluster); err != nil {
				return fmt.Errorf("populateInitialConfigEntries[%s]: %w", cluster.Name, err)
			}
		}

		// Create tokens for all of the agents to use for anti-entropy.
		//
		// NOTE: this will cause the servers to roll to pick up the change to
		// the acl{tokens{agent=XXX}}} section.
		if err := s.createAgentTokens(cluster); err != nil {
			return fmt.Errorf("createAgentTokens[%s]: %w", cluster.Name, err)
		}
	}

	return nil
}

func (s *Sprawl) waitForLocalWrites(cluster *topology.Cluster, token string) {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)
	tryKV := func() error {
		_, err := client.KV().Put(&api.KVPair{
			Key:   "local-test",
			Value: []byte("payload-for-local-test-in-" + cluster.Name),
		}, nil)
		return err
	}
	tryAP := func() error {
		if !cluster.Enterprise {
			return nil
		}
		_, _, err := client.Partitions().Create(context.Background(), &api.Partition{
			Name: "placeholder",
		}, &api.WriteOptions{Token: token})
		return err
	}

	start := time.Now()
	for attempts := 0; ; attempts++ {
		if err := tryKV(); err != nil {
			err = TruncateSquidError(err)
			logger.Warn("local kv write failed; something is not ready yet", "error", err)
			time.Sleep(500 * time.Millisecond)
			continue
		} else {
			dur := time.Since(start)
			logger.Info("local kv write success", "elapsed", dur, "retries", attempts)
		}

		break
	}

	if cluster.Enterprise {
		start = time.Now()
		for attempts := 0; ; attempts++ {
			if err := tryAP(); err != nil {
				// err = TruncateSquidError(err)
				logger.Warn("local partition write failed; something is not ready yet", "error", err)
				time.Sleep(500 * time.Millisecond)
				continue
			} else {
				dur := time.Since(start)
				logger.Info("local partition write success", "elapsed", dur, "retries", attempts)
			}

			break
		}
	}
}

func (s *Sprawl) waitForClientAntiEntropyOnce(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	var (
		queryOptionList = cluster.PartitionQueryOptionsList()
		start           = time.Now()
		cc              = client.Catalog()
	)
	for {
		// Enumerate all of the nodes that are currently in the catalog. This
		// will overmatch including things like fake nodes for agentless but
		// that's ok.
		current := make(map[topology.NodeID]*api.Node)
		for _, queryOpts := range queryOptionList {
			nodes, _, err := cc.Nodes(queryOpts)
			if err != nil {
				return err
			}
			for _, node := range nodes {
				nid := topology.NewNodeID(node.Node, node.Partition)
				current[nid] = node
			}
		}

		// See if we have them all.
		var stragglers []topology.NodeID
		for _, node := range cluster.Nodes {
			if !node.IsAgent() || node.Disabled {
				continue
			}
			nid := node.CatalogID()

			got, ok := current[nid]
			if ok && len(got.TaggedAddresses) > 0 {
				// this is a field that is not updated just due to serf reconcile
				continue
			}

			stragglers = append(stragglers, nid)
		}

		if len(stragglers) == 0 {
			dur := time.Since(start)
			logger.Info("all nodes have posted node updates, so first anti-entropy has happened", "elapsed", dur)
			return nil
		}
		logger.Info("not all client nodes have posted node updates yet", "nodes", stragglers)

		time.Sleep(1 * time.Second)
	}
}

func newGossipKey() (string, error) {
	key := make([]byte, 16)
	n, err := rand.Reader.Read(key)
	if err != nil {
		return "", fmt.Errorf("Error reading random data: %s", err)
	}
	if n != 16 {
		return "", fmt.Errorf("Couldn't read enough entropy. Generate more entropy!")
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
