package instances

import (
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestUsageInstancesCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	// Add another 2 services for testing
	if err := a.Client().Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name:    "testing",
		Port:    8080,
		Address: "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Client().Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name:    "testing2",
		Port:    8081,
		Address: "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}

	ui := cli.NewMockUi()
	c := New(ui)
	args := []string{
		"-http-addr=" + a.HTTPAddr(),
	}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
	}
	output := ui.OutputWriter.String()
	require.Contains(t, output, "Billable Service Instances Total: 2")
}
