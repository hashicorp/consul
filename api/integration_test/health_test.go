package integration_test

import (
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

//go:generate cstestgen
func cstestAPI_HealthNode(t *testing.T, c *api.Client, s TestServerI) {
	t.Parallel()
	defer s.Stop()

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
