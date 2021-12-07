package policycreate

import (
	"encoding/json"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyCreateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestPolicyCreateCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	require := require.New(t)

	testDir := testutil.TempDir(t, "acl")

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

	rules := []byte("service \"\" { policy = \"write\" }")
	err := ioutil.WriteFile(testDir+"/rules.hcl", rules, 0644)
	require.NoError(err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-name=foobar",
		"-rules=@" + testDir + "/rules.hcl",
	}

	code := cmd.Run(args)
	require.Equal(code, 0)
	require.Empty(ui.ErrorWriter.String())
}

func TestPolicyCreateCommand_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	require := require.New(t)

	testDir := testutil.TempDir(t, "acl")

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

	rules := []byte("service \"\" { policy = \"write\" }")
	err := ioutil.WriteFile(testDir+"/rules.hcl", rules, 0644)
	require.NoError(err)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-name=foobar",
		"-rules=@" + testDir + "/rules.hcl",
		"-format=json",
	}

	code := cmd.Run(args)
	require.Equal(code, 0)
	require.Empty(ui.ErrorWriter.String())

	var jsonOutput json.RawMessage
	err = json.Unmarshal([]byte(ui.OutputWriter.String()), &jsonOutput)
	assert.NoError(t, err)
}
