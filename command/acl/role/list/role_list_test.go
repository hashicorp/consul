// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package rolelist

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

func TestRoleListCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRoleListCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

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

	var roleIDs []string

	// Create a couple roles to list
	client := a.Client()
	svcids := []*api.ACLServiceIdentity{
		{ServiceName: "fake"},
	}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("test-role-%d", i)

		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{Name: name, ServiceIdentities: svcids},
			&api.WriteOptions{Token: "root"},
		)
		roleIDs = append(roleIDs, role.ID)

		require.NoError(t, err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
	}

	code := cmd.Run(args)
	require.Equal(t, code, 0)
	require.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()

	for i, v := range roleIDs {
		require.Contains(t, output, fmt.Sprintf("test-role-%d", i))
		require.Contains(t, output, v)
	}
}

func TestRoleListCommand_JSON(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

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

	var roleIDs []string

	// Create a couple roles to list
	client := a.Client()
	svcids := []*api.ACLServiceIdentity{
		{ServiceName: "fake"},
	}
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("test-role-%d", i)

		role, _, err := client.ACL().RoleCreate(
			&api.ACLRole{Name: name, ServiceIdentities: svcids},
			&api.WriteOptions{Token: "root"},
		)
		roleIDs = append(roleIDs, role.ID)

		require.NoError(t, err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-token=root",
		"-format=json",
	}

	code := cmd.Run(args)
	require.Equal(t, code, 0)
	require.Empty(t, ui.ErrorWriter.String())
	output := ui.OutputWriter.String()

	for i, v := range roleIDs {
		require.Contains(t, output, fmt.Sprintf("test-role-%d", i))
		require.Contains(t, output, v)
	}

	var jsonOutput json.RawMessage
	err := json.Unmarshal([]byte(output), &jsonOutput)
	assert.NoError(t, err)
}
