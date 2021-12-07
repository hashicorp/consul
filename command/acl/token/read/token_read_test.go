package tokenread

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenReadCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenReadCommand_Pretty(t *testing.T) {
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

	// Create a token
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-id=" + token.AccessorID,
	}

	code := cmd.Run(args)
	assert.Equal(code, 0)
	assert.Empty(ui.ErrorWriter.String())

	output := ui.OutputWriter.String()
	assert.Contains(output, fmt.Sprintf("test"))
	assert.Contains(output, token.AccessorID)
	assert.Contains(output, token.SecretID)
}

func TestTokenReadCommand_JSON(t *testing.T) {
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

	// Create a token
	client := a.Client()

	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test"},
		&api.WriteOptions{Token: "root"},
	)
	assert.NoError(err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-id=" + token.AccessorID,
		"-format=json",
	}

	code := cmd.Run(args)
	assert.Equal(code, 0)
	assert.Empty(ui.ErrorWriter.String())

	var jsonOutput json.RawMessage
	err = json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
	require.NoError(t, err, "token unmarshalling error")
}
