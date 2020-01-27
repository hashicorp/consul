package policylist

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestPolicyListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestPolicyListCommand(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t, t.Name(), `
	primary_datacenter = "dc1"
	acl {
		enabled = true
		tokens {
			master = "root"
		}
	}`)

	a.Agent.LogWriter = logger.NewLogWriter(512)

	defer a.Shutdown()
	testrpc.WaitForLeader(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	cmd := New(ui)

	var policyIDs []string

	// Create a couple polices to list
	client := a.Client()
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("test-policy-%d", i)

		policy, _, err := client.ACL().PolicyCreate(
			&api.ACLPolicy{Name: name},
			&api.WriteOptions{Token: "root"},
		)
		policyIDs = append(policyIDs, policy.ID)

		assert.NoError(err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	code := cmd.Run(args)
	assert.Equal(code, 0)
	assert.Empty(ui.ErrorWriter.String())
	output := ui.OutputWriter.String()

	for i, v := range policyIDs {
		assert.Contains(output, fmt.Sprintf("test-policy-%d", i))
		assert.Contains(output, v)
	}
}
