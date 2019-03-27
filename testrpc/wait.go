package testrpc

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

type rpcFn func(string, interface{}, interface{}) error

// WaitForLeader ensures we have a leader and a node registration.
func WaitForLeader(t *testing.T, rpc rpcFn, dc string) {
	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{Datacenter: dc}
		if err := rpc("Catalog.ListNodes", args, &out); err != nil {
			r.Fatalf("Catalog.ListNodes failed: %v", err)
		}
		if !out.QueryMeta.KnownLeader {
			r.Fatalf("No leader")
		}
		if out.Index < 2 {
			r.Fatalf("Consul index should be at least 2")
		}
	})
}

// WaitUntilNoLeader ensures no leader is present, useful for testing lost leadership.
func WaitUntilNoLeader(t *testing.T, rpc rpcFn, dc string) {
	var out structs.IndexedNodes
	retry.Run(t, func(r *retry.R) {
		args := &structs.DCSpecificRequest{Datacenter: dc}
		if err := rpc("Catalog.ListNodes", args, &out); err == nil {
			r.Fatalf("It still has a leader: %#v", out)
		}
		if out.QueryMeta.KnownLeader {
			r.Fatalf("Has still a leader")
		}
	})
}

type waitOption struct {
	Token string
}

func WithToken(token string) waitOption {
	return waitOption{Token: token}
}

// WaitForTestAgent ensures we have a node with serfHealth check registered
func WaitForTestAgent(t *testing.T, rpc rpcFn, dc string, options ...waitOption) {
	var nodes structs.IndexedNodes
	var checks structs.IndexedHealthChecks

	// first extra arg is an optional acl token
	var token string
	for _, opt := range options {
		if opt.Token != "" {
			token = opt.Token
		}
	}

	retry.Run(t, func(r *retry.R) {
		dcReq := &structs.DCSpecificRequest{
			Datacenter:   dc,
			QueryOptions: structs.QueryOptions{Token: token},
		}
		if err := rpc("Catalog.ListNodes", dcReq, &nodes); err != nil {
			r.Fatalf("Catalog.ListNodes failed: %v", err)
		}
		if len(nodes.Nodes) == 0 {
			r.Fatalf("No registered nodes")
		}

		// This assumes that there is a single agent per dc, typically a TestAgent
		nodeReq := &structs.NodeSpecificRequest{
			Datacenter:   dc,
			Node:         nodes.Nodes[0].Node,
			QueryOptions: structs.QueryOptions{Token: token},
		}
		if err := rpc("Health.NodeChecks", nodeReq, &checks); err != nil {
			r.Fatalf("Health.NodeChecks failed: %v", err)
		}

		var found bool
		for _, check := range checks.HealthChecks {
			if check.CheckID == "serfHealth" {
				found = true
				break
			}
		}
		if !found {
			r.Fatalf("serfHealth check not found")
		}
	})
}
