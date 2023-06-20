package sprawl

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/consul-topology/sprawl/internal/secrets"
	"github.com/hashicorp/consul/consul-topology/topology"
)

// TODO: fix this by checking that a token/policy works on ALL servers before
// returning from create.
func isACLNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), `ACL not found`)
}

func (s *Sprawl) bootstrapACLs(cluster string) error {
	var (
		client    = s.clients[cluster]
		logger    = s.logger.With("cluster", cluster)
		mgmtToken = s.secrets.ReadGeneric(cluster, secrets.BootstrapToken)
	)

	ac := client.ACL()

	if mgmtToken != "" {
	NOT_BOOTED:
		ready, err := s.isACLBootstrapped(cluster, client)
		if err != nil {
			return fmt.Errorf("error checking if the acl system is bootstrapped: %w", err)
		} else if !ready {
			logger.Warn("ACL system is not ready yet")
			time.Sleep(250 * time.Millisecond)
			goto NOT_BOOTED
		}

	TRYAGAIN:
		// check to see if it works
		_, _, err = ac.TokenReadSelf(&api.QueryOptions{Token: mgmtToken})
		if err != nil {
			if isACLNotBootstrapped(err) {
				logger.Warn("system is rebooting", "error", err)
				time.Sleep(250 * time.Millisecond)
				goto TRYAGAIN
			}

			return fmt.Errorf("management token no longer works: %w", err)
		}

		logger.Info("current management token", "token", mgmtToken)
		return nil
	}

TRYAGAIN2:
	logger.Info("bootstrapping ACLs")
	tok, _, err := ac.Bootstrap()
	if err != nil {
		if isACLNotBootstrapped(err) {
			logger.Warn("system is rebooting", "error", err)
			time.Sleep(250 * time.Millisecond)
			goto TRYAGAIN2
		}
		return err
	}
	mgmtToken = tok.SecretID
	s.secrets.SaveGeneric(cluster, secrets.BootstrapToken, mgmtToken)

	logger.Info("current management token", "token", mgmtToken)

	return nil

}

func isACLNotBootstrapped(err error) bool {
	switch {
	case strings.Contains(err.Error(), "ACL system must be bootstrapped before making any requests that require authorization"):
		return true
	case strings.Contains(err.Error(), "The ACL system is currently in legacy mode"):
		return true
	}
	return false
}

func (s *Sprawl) isACLBootstrapped(cluster string, client *api.Client) (bool, error) {
	policy, _, err := client.ACL().PolicyReadByName("global-management", &api.QueryOptions{
		Token: s.secrets.ReadGeneric(cluster, secrets.BootstrapToken),
	})
	if err != nil {
		if strings.Contains(err.Error(), "Unexpected response code: 403 (ACL not found)") {
			return false, nil
		} else if isACLNotBootstrapped(err) {
			return false, nil
		}
		return false, err
	}
	return policy != nil, nil
}

func (s *Sprawl) createAnonymousToken(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	if err := s.createAnonymousPolicy(cluster); err != nil {
		return err
	}

	token, err := CreateOrUpdateToken(client, anonymousToken())
	if err != nil {
		return err
	}

	logger.Info("created anonymous token",
		"token", token.SecretID,
	)

	return nil
}

func (s *Sprawl) createAnonymousPolicy(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	op, err := CreateOrUpdatePolicy(client, anonymousPolicy(cluster.Enterprise))
	if err != nil {
		return err
	}

	logger.Info("created anonymous policy",
		"policy-name", op.Name,
		"policy-id", op.ID,
	)

	return nil
}

func (s *Sprawl) createAgentTokens(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	for _, node := range cluster.Nodes {
		// NOTE: always create tokens even for disabled nodes.
		if !node.IsAgent() {
			continue
		}

		if tok := s.secrets.ReadAgentToken(cluster.Name, node.ID()); tok == "" {
			token, err := CreateOrUpdateToken(client, tokenForNode(node, cluster.Enterprise))
			if err != nil {
				return err
			}

			logger.Info("created agent token",
				"node", node.ID(),
				"token", token.SecretID,
			)

			s.secrets.SaveAgentToken(cluster.Name, node.ID(), token.SecretID)
		}
	}

	return nil
}

// Create a policy to allow super permissive catalog reads across namespace
// boundaries.
func (s *Sprawl) createCrossNamespaceCatalogReadPolicies(cluster *topology.Cluster, partition string) error {
	if !cluster.Enterprise {
		return nil
	}

	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	op, err := CreateOrUpdatePolicy(client, policyForCrossNamespaceRead(partition))
	if err != nil {
		return err
	}

	logger.Info("created cross-ns-catalog-read policy",
		"policy-name", op.Name,
		"policy-id", op.ID,
		"partition", partition,
	)

	return nil
}

func (s *Sprawl) createAllServiceTokens() error {
	for _, cluster := range s.topology.Clusters {
		if err := s.createServiceTokens(cluster); err != nil {
			return fmt.Errorf("createServiceTokens[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) createServiceTokens(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	sids := make(map[topology.ServiceID]struct{})
	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Services) == 0 || node.Disabled {
			continue
		}

		for _, svc := range node.Services {
			sid := svc.ID

			if _, done := sids[sid]; done {
				continue
			}

			var overridePolicy *api.ACLPolicy
			if svc.IsMeshGateway {
				var err error
				overridePolicy, err = CreateOrUpdatePolicy(client, policyForMeshGateway(svc, cluster.Enterprise))
				if err != nil {
					return fmt.Errorf("could not create policy: %w", err)
				}
			}

			token, err := CreateOrUpdateToken(client, tokenForService(svc, overridePolicy, cluster.Enterprise))
			if err != nil {
				return fmt.Errorf("could not create token: %w", err)
			}

			logger.Info("created service token",
				"service", svc.ID.Name,
				"namespace", svc.ID.Namespace,
				"partition", svc.ID.Partition,
				"token", token.SecretID,
			)

			s.secrets.SaveServiceToken(cluster.Name, sid, token.SecretID)

			sids[sid] = struct{}{}
		}
	}

	return nil
}

func CreateOrUpdateToken(client *api.Client, t *api.ACLToken) (*api.ACLToken, error) {
	ac := client.ACL()

	currentToken, err := getTokenByDescription(client, t.Description, &api.QueryOptions{
		Partition: t.Partition,
		Namespace: t.Namespace,
	})
	if err != nil {
		return nil, err
	} else if currentToken != nil {
		t.AccessorID = currentToken.AccessorID
		t.SecretID = currentToken.SecretID
	}

	if t.AccessorID != "" {
		t, _, err = ac.TokenUpdate(t, nil)
	} else {
		t, _, err = ac.TokenCreate(t, nil)
	}
	if err != nil {
		return nil, err
	}
	return t, nil
}

func getTokenByDescription(client *api.Client, description string, opts *api.QueryOptions) (*api.ACLToken, error) {
	ac := client.ACL()
	tokens, _, err := ac.TokenList(opts)
	if err != nil {
		return nil, err
	}

	for _, tokenEntry := range tokens {
		if tokenEntry.Description == description {
			token, _, err := ac.TokenRead(tokenEntry.AccessorID, opts)
			if err != nil {
				return nil, err
			}

			return token, nil
		}
	}
	return nil, nil
}

func CreateOrUpdatePolicy(client *api.Client, p *api.ACLPolicy) (*api.ACLPolicy, error) {
	ac := client.ACL()

	currentPolicy, _, err := ac.PolicyReadByName(p.Name, &api.QueryOptions{
		Partition: p.Partition,
		Namespace: p.Namespace,
	})

	// There is a quirk about Consul 1.14.x, where: if reading a policy yields
	// an empty result, we return "ACL not found". It's safe to ignore this here,
	// because if the Client's ACL token truly doesn't exist, then the create fails below.
	if err != nil && !strings.Contains(err.Error(), "ACL not found") {
		return nil, err
	} else if currentPolicy != nil {
		p.ID = currentPolicy.ID
	}

	if p.ID != "" {
		p, _, err = ac.PolicyUpdate(p, nil)
	} else {
		p, _, err = ac.PolicyCreate(p, nil)
	}

	if err != nil {
		return nil, err
	}
	return p, nil
}
