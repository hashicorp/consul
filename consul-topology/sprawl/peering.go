package sprawl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/consul-topology/topology"
)

// TODO: this is definitely a grpc resolver/balancer issue to look into
const grpcWeirdError = `transport: Error while dialing failed to find Consul server for global address`

func isWeirdGRPCError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), grpcWeirdError)
}

func (s *Sprawl) initPeerings() error {
	// TODO: wait until services are healthy? wait until mesh gateways work?
	// if err := s.generator.Generate(tfgen.StepPeering); err != nil {
	// 	return fmt.Errorf("generator[peering]: %w", err)
	// }

	var (
		logger = s.logger.Named("peering")
		_      = logger
	)

	for _, peering := range s.topology.Peerings {
		dialingCluster, ok := s.topology.Clusters[peering.Dialing.Name]
		if !ok {
			return fmt.Errorf("peering references dialing cluster that does not exist: %s", peering.String())
		}
		acceptingCluster, ok := s.topology.Clusters[peering.Accepting.Name]
		if !ok {
			return fmt.Errorf("peering references accepting cluster that does not exist: %s", peering.String())
		}

		var (
			dialingClient   = s.clients[dialingCluster.Name]
			acceptingClient = s.clients[acceptingCluster.Name]
		)

		// TODO: allow for use of ServerExternalAddresses

		req1 := api.PeeringGenerateTokenRequest{
			PeerName: peering.Accepting.PeerName,
		}
		if acceptingCluster.Enterprise {
			req1.Partition = peering.Accepting.Partition
		}

	GENTOKEN:
		resp, _, err := acceptingClient.Peerings().GenerateToken(context.Background(), req1, nil)
		if err != nil {
			if isWeirdGRPCError(err) {
				time.Sleep(50 * time.Millisecond)
				goto GENTOKEN
			}
			return fmt.Errorf("error generating peering token for %q: %w", peering.String(), err)
		}

		peeringToken := resp.PeeringToken
		logger.Info("generated peering token", "peering", peering.String())

		req2 := api.PeeringEstablishRequest{
			PeerName:     peering.Dialing.PeerName,
			PeeringToken: peeringToken,
		}
		if dialingCluster.Enterprise {
			req2.Partition = peering.Dialing.Partition
		}

		logger.Info("establishing peering with token", "peering", peering.String())
	ESTABLISH:
		_, _, err = dialingClient.Peerings().Establish(context.Background(), req2, nil)
		if err != nil {
			if isWeirdGRPCError(err) {
				time.Sleep(50 * time.Millisecond)
				goto ESTABLISH
			}
			return fmt.Errorf("error establishing peering with token for %q: %w", peering.String(), err)
		}

		logger.Info("peering established", "peering", peering.String())
	}

	return nil
}

func (s *Sprawl) waitForPeeringEstablishment() error {
	var (
		logger = s.logger.Named("peering")
	)

	for _, peering := range s.topology.Peerings {
		dialingCluster, ok := s.topology.Clusters[peering.Dialing.Name]
		if !ok {
			return fmt.Errorf("peering references dialing cluster that does not exist: %s", peering.String())
		}
		acceptingCluster, ok := s.topology.Clusters[peering.Accepting.Name]
		if !ok {
			return fmt.Errorf("peering references accepting cluster that does not exist: %s", peering.String())
		}

		var (
			dialingClient   = s.clients[dialingCluster.Name]
			acceptingClient = s.clients[acceptingCluster.Name]

			dialingLogger = logger.With(
				"cluster", dialingCluster.Name,
				"peering", peering.String(),
			)
			acceptingLogger = logger.With(
				"cluster", acceptingCluster.Name,
				"peering", peering.String(),
			)
		)

		s.checkPeeringDirection(dialingLogger, dialingClient, peering.Dialing, dialingCluster.Enterprise)
		s.checkPeeringDirection(acceptingLogger, acceptingClient, peering.Accepting, acceptingCluster.Enterprise)
	}
	return nil
}

func (s *Sprawl) checkPeeringDirection(logger hclog.Logger, client *api.Client, pc topology.PeerCluster, enterprise bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for {
		opts := &api.QueryOptions{}
		if enterprise {
			opts.Partition = pc.Partition
		}
		res, _, err := client.Peerings().Read(ctx, pc.PeerName, opts)
		if isWeirdGRPCError(err) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if err != nil {
			logger.Info("error looking up peering", "error", err)
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if res == nil {
			logger.Info("peering not found")
			time.Sleep(100 * time.Millisecond)
			continue
		}

		if res.State == api.PeeringStateActive {
			logger.Info("peering is active")
			return
		}
		logger.Info("peering not active yet", "state", res.State)
		time.Sleep(500 * time.Millisecond)
	}
}
