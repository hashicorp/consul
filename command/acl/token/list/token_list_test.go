package tokenlist

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

func TestTokenListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenListCommand(t *testing.T) {
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

	var tokenIds []string

	// Create a couple tokens to list
	client := a.Client()
	for i := 0; i < 5; i++ {
		description := fmt.Sprintf("test token %d", i)

		token, _, err := client.ACL().TokenCreate(
			&api.ACLToken{Description: description},
			&api.WriteOptions{Token: "root"},
		)
		tokenIds = append(tokenIds, token.AccessorID)

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

	for i, v := range tokenIds {
		assert.Contains(output, fmt.Sprintf("test token %d", i))
		assert.Contains(output, v)
	}
}
