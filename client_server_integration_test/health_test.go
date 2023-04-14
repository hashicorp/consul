package integration_test

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

//go:generate cstestgen
type cstestAPI_HealthNode struct {
}

func (cstest *cstestAPI_HealthNode) assemble(t *testing.T, c *api.Client, s TestServerI) {
	// nothing
}

func (cstest *cstestAPI_HealthNode) act(t *testing.T, c *api.Client, s TestServerI) {
	// nothing
}

func (cstest *cstestAPI_HealthNode) assert(t *testing.T, c *api.Client, s TestServerI) {
	agent := c.Agent()
	health := c.Health()

	info, err := agent.Self()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	name := info["Config"]["NodeName"].(string)
	retry.Run(t, func(r *retry.R) {
		checks, meta, err := health.Node(name, nil)
		if err != nil {
			r.Fatal(err)
		}
		if meta.LastIndex == 0 {
			r.Fatalf("bad: %v", meta)
		}
		if len(checks) == 0 {
			r.Fatalf("bad: %v", checks)
		}
	})
}
