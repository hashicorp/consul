package policydelete

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestPolicyDeleteCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestPolicyDeleteCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	assert := assert.New(t)

	a := agent.NewTestAgent(t, `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			initial_management = "root"
		}
	}`)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	// Create a policy
	client := a.Client()

	policy, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-id=" + policy.ID,
	}

	code := cmd.Run(args)
	assert.Equal(code, 0)
	assert.Empty(ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	assert.Contains(output, fmt.Sprintf("deleted successfully"))
	assert.Contains(output, policy.ID)

	_, _, err = client.ACL().PolicyRead(
		policy.ID,
		&api.QueryOptions{Token: "root"},
	)
	assert.EqualError(err, "Unexpected response code: 403 (ACL not found)")
}
