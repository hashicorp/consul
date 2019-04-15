package rolelist

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
	"github.com/stretchr/testify/require"
)

func TestRoleListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleListCommand(t *testing.T) {
	t.Parallel()
	require := require.New(t)

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

	var roleIDs []string

	// Create a couple roles to list
	client := a.Client()
	svcids := []*api.ACLServiceIdentity{
		&api.ACLServiceIdentity{ServiceName: "fake"},
	}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("test-role-%d", i)

		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{Name: name, ServiceIdentities: svcids},
			&api.WriteOptions{Token: "root"},
		)
		roleIDs = append(roleIDs, role.ID)

		require.NoError(err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	code := cmd.Run(args)
	require.Equal(code, 0)
	require.Empty(ui.ErrorWriter.String())
	output := ui.OutputWriter.String()

	for i, v := range roleIDs {
		require.Contains(output, fmt.Sprintf("test-role-%d", i))
		require.Contains(output, v)
	}
}
