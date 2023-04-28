package sprawl

import (
	"fmt"

	"github.com/hashicorp/consul/api"

	"github.com/hashicorp/consul-topology/topology"
)

func policyForAgentRead(partition string) *api.ACLPolicy {
	return &api.ACLPolicy{
		Name:        "wildcard-agent-read",
		Description: "wildcard-agent-read",
		Partition:   partition,
		Rules: `
agent_prefix "" {
  policy = "read"
}
`,
	}
}

func policyForCrossNamespaceRead(partition string) *api.ACLPolicy {
	return &api.ACLPolicy{
		Name:        "cross-ns-catalog-read",
		Description: "cross-ns-catalog-read",
		Partition:   partition,
		Rules: fmt.Sprintf(`
partition %[1]q {
  namespace_prefix "" {
    node_prefix ""    { policy = "read" }
    service_prefix "" { policy = "read" }
  }
}
`, partition),
	}
}

func tokenForNode(node *topology.Node, enterprise bool) *api.ACLToken {
	nid := node.ID()

	tokenName := "agent--" + nid.ACLString()

	token := &api.ACLToken{
		Description: tokenName,
		Local:       false,
		NodeIdentities: []*api.ACLNodeIdentity{{
			NodeName:   node.PodName(),
			Datacenter: node.Datacenter,
		}},
	}
	if enterprise {
		token.Partition = node.Partition
		token.Namespace = "default"
	}
	return token
}

func tokenForService(svc *topology.Service, overridePolicy *api.ACLPolicy, enterprise bool) *api.ACLToken {
	token := &api.ACLToken{
		Description: "service--" + svc.ID.ACLString(),
		Local:       false,
	}
	if overridePolicy != nil {
		token.Policies = []*api.ACLTokenPolicyLink{{ID: overridePolicy.ID}}
	} else {
		token.ServiceIdentities = []*api.ACLServiceIdentity{{
			ServiceName: svc.ID.Name,
		}}
	}

	// TODO: remove this hack
	token.Policies = append(token.Policies, &api.ACLTokenPolicyLink{Name: "wildcard-agent-read"})

	if enterprise {
		token.Namespace = svc.ID.Namespace
		token.Partition = svc.ID.Partition
	}

	return token
}

func policyForMeshGateway(svc *topology.Service, enterprise bool) *api.ACLPolicy {
	policyName := "mesh-gateway--" + svc.ID.ACLString()

	policy := &api.ACLPolicy{
		Name:        policyName,
		Description: policyName,
	}
	if enterprise {
		policy.Partition = svc.ID.Partition
		policy.Namespace = "default"
	}

	if enterprise {
		policy.Rules = `
namespace_prefix "" {
  service "mesh-gateway" {
    policy = "write"
  }
  service_prefix "" {
    policy = "read"
  }
  node_prefix "" {
    policy = "read"
  }
}
agent_prefix "" {
  policy = "read"
}
# for peering
mesh = "write"
peering = "read"
`
	} else {
		policy.Rules = `
service "mesh-gateway" {
  policy = "write"
}
service_prefix "" {
  policy = "read"
}
node_prefix "" {
  policy = "read"
}
agent_prefix "" {
  policy = "read"
}
# for peering
mesh = "write"
peering = "read"
`
	}

	return policy
}
