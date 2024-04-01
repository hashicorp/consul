// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul/testing/deployer/sprawl/internal/secrets"
	"github.com/hashicorp/consul/testing/deployer/topology"
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

		logger.Debug("current management token", "token", mgmtToken)
		return nil
	}

TRYAGAIN2:
	logger.Info("bootstrapping ACLs")
	tok, _, err := ac.Bootstrap()
	if err != nil {
		if isACLNotBootstrapped(err) {
			logger.Debug("system is rebooting", "error", err)
			time.Sleep(250 * time.Millisecond)
			goto TRYAGAIN2
		}
		return err
	}
	mgmtToken = tok.SecretID
	s.secrets.SaveGeneric(cluster, secrets.BootstrapToken, mgmtToken)

	logger.Debug("current management token", "token", mgmtToken)

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

	logger.Debug("created anonymous token",
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

	logger.Debug("created anonymous policy",
		"policy-name", op.Name,
		"policy-id", op.ID,
	)

	return nil
}

// assignAgentJoinPolicyToAnonymousToken is used only for version prior to agent token
func (s *Sprawl) assignAgentJoinPolicyToAnonymousToken(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
	)

	acl := client.ACL()
	anonymousTok, _, err := acl.TokenRead(anonymousTokenAccessorID, &api.QueryOptions{})
	if err != nil {
		return nil
	}

	rule := `
service_prefix "" {
	policy = "read"
}
	
agent_prefix "" {
	policy = "read"
}
	
node_prefix "" {
	policy = "write"
}

operator = "write"
`
	policy, _, err := acl.PolicyCreate(
		&api.ACLPolicy{
			Name:  "client-join-policy",
			Rules: rule,
		},
		&api.WriteOptions{},
	)

	if err != nil {
		return err
	}

	anonymousTok.Policies = append(anonymousTok.Policies,
		&api.ACLLink{
			Name: policy.Name,
		},
	)
	_, _, err = acl.TokenUpdate(anonymousTok, &api.WriteOptions{})
	if err != nil {
		return nil
	}

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

		if node.Images.GreaterThanVersion(topology.MinVersionAgentTokenPartition) {
			if tok := s.secrets.ReadAgentToken(cluster.Name, node.ID()); tok == "" {
				token, err := CreateOrUpdateToken(client, tokenForNode(node, cluster.Enterprise))
				if err != nil {
					return fmt.Errorf("node %s: %w", node.Name, err)
				}

				logger.Debug("created agent token",
					"node", node.ID(),
					"token", token.SecretID,
				)

				s.secrets.SaveAgentToken(cluster.Name, node.ID(), token.SecretID)
			}
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

	logger.Debug("created cross-ns-catalog-read policy",
		"policy-name", op.Name,
		"policy-id", op.ID,
		"partition", partition,
	)

	return nil
}

func (s *Sprawl) createAllWorkloadTokens() error {
	for _, cluster := range s.topology.Clusters {
		if err := s.createWorkloadTokens(cluster); err != nil {
			return fmt.Errorf("createWorkloadTokens[%s]: %w", cluster.Name, err)
		}
	}
	return nil
}

func (s *Sprawl) createWorkloadTokens(cluster *topology.Cluster) error {
	var (
		client = s.clients[cluster.Name]
		logger = s.logger.With("cluster", cluster.Name)
	)

	workloadIDs := make(map[topology.ID]struct{})
	for _, node := range cluster.Nodes {
		if !node.RunsWorkloads() || len(node.Workloads) == 0 || node.Disabled {
			continue
		}

		for _, wrk := range node.Workloads {
			if _, done := workloadIDs[wrk.ID]; done {
				continue
			}

			var overridePolicy *api.ACLPolicy
			if wrk.IsMeshGateway {
				var err error
				overridePolicy, err = CreateOrUpdatePolicy(client, policyForMeshGateway(wrk, cluster.Enterprise))
				if err != nil {
					return fmt.Errorf("could not create policy: %w", err)
				}
			}

			token, err := CreateOrUpdateToken(client, tokenForWorkload(wrk, overridePolicy, cluster.Enterprise))
			if err != nil {
				return fmt.Errorf("could not create token: %w", err)
			}

			logger.Debug("created workload token",
				"workload", wrk.ID.Name,
				"namespace", wrk.ID.Namespace,
				"partition", wrk.ID.Partition,
				"token", token.SecretID,
			)

			s.secrets.SaveWorkloadToken(cluster.Name, wrk.ID, token.SecretID)

			workloadIDs[wrk.ID] = struct{}{}
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
