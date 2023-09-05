// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package tokenclone

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testrpc"
)

func parseCloneOutput(t *testing.T, output string) *api.ACLToken {
	re := regexp.MustCompile("AccessorID:       ([a-zA-Z0-9\\-]{36})\n" +
		"SecretID:         ([a-zA-Z0-9\\-]{36})\n" +
		"(?:Partition:        default\n)?" +
		"(?:Namespace:        default\n)?" +
		"Description:      ([^\n]*)\n" +
		"Local:            (true|false)\n" +
		"Create Time:      ([^\n]+)\n" +
		"Policies:\n" +
		"(   [a-zA-Z0-9\\-]{36} - [^\n]+\n)*")

	submatches := re.FindStringSubmatch(output)
	require.Lenf(t, submatches, 7, "Didn't match: %q", output)

	local, err := strconv.ParseBool(submatches[4])
	require.NoError(t, err)

	token := &api.ACLToken{
		AccessorID:  submatches[1],
		SecretID:    submatches[2],
		Description: submatches[3],
		Local:       local,
	}

	if len(submatches[6]) > 0 {
		policyRe := regexp.MustCompile("   ([a-zA-Z0-9\\-]{36}) - ([^\n]+)")
		for _, m := range policyRe.FindAllStringSubmatch(submatches[6], -1) {
			token.Policies = append(token.Policies, &api.ACLTokenPolicyLink{ID: m[1], Name: m[2]})
		}
	}

	return token
}

func TestTokenCloneCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestTokenCloneCommand_Pretty(t *testing.T) {
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

	// Create a policy
	client := a.Client()

	_, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create a token
	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test", Policies: []*api.ACLTokenPolicyLink{{Name: "test-policy"}}},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// clone with description
	t.Run("Description", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-description=test cloned",
		}

		code := cmd.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, code, 0)

		cloned := parseCloneOutput(t, ui.OutputWriter.String())

		require.Equal(t, "test cloned", cloned.Description)
		require.Len(t, cloned.Policies, 1)

		apiToken, _, err := client.ACL().TokenRead(
			cloned.AccessorID,
			&api.QueryOptions{Token: "root"},
		)

		require.NoError(t, err)
		require.NotNil(t, apiToken)

		require.Equal(t, cloned.AccessorID, apiToken.AccessorID)
		require.Equal(t, cloned.SecretID, apiToken.SecretID)
		require.Equal(t, cloned.Description, apiToken.Description)
		require.Equal(t, cloned.Local, apiToken.Local)
		require.Equal(t, cloned.Policies, apiToken.Policies)
	})

	// clone without description
	t.Run("Without Description", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())

		cloned := parseCloneOutput(t, ui.OutputWriter.String())

		require.Equal(t, "test", cloned.Description)
		require.Len(t, cloned.Policies, 1)

		apiToken, _, err := client.ACL().TokenRead(
			cloned.AccessorID,
			&api.QueryOptions{Token: "root"},
		)

		require.NoError(t, err)
		require.NotNil(t, apiToken)

		require.Equal(t, cloned.AccessorID, apiToken.AccessorID)
		require.Equal(t, cloned.SecretID, apiToken.SecretID)
		require.Equal(t, cloned.Description, apiToken.Description)
		require.Equal(t, cloned.Local, apiToken.Local)
		require.Equal(t, cloned.Policies, apiToken.Policies)
	})
}

func TestTokenCloneCommand_JSON(t *testing.T) {
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

	// Create a policy
	client := a.Client()

	_, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// create a token
	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test", Policies: []*api.ACLTokenPolicyLink{{Name: "test-policy"}}},
		&api.WriteOptions{Token: "root"},
	)
	require.NoError(t, err)

	// clone with description
	t.Run("Description", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-description=test cloned",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, code, 0)

		output := ui.OutputWriter.String()
		var jsonOutput json.RawMessage
		err = json.Unmarshal([]byte(output), &jsonOutput)
		assert.NoError(t, err)
	})

	// clone without description
	t.Run("Without Description", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-accessor-id=" + token.AccessorID,
			"-token=root",
			"-format=json",
		}

		code := cmd.Run(args)
		require.Empty(t, ui.ErrorWriter.String())
		require.Equal(t, code, 0)

		output := ui.OutputWriter.String()
		var jsonOutput json.RawMessage
		err = json.Unmarshal([]byte(output), &jsonOutput)
		assert.NoError(t, err)
	})
}
