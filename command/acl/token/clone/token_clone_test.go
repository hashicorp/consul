package tokenclone

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func parseCloneOutput(t *testing.T, output string) *api.ACLToken {
	// This will only work for non-legacy tokens
	re := regexp.MustCompile("Token cloned successfully.\n" +
		"AccessorID:       ([a-zA-Z0-9\\-]{36})\n" +
		"SecretID:         ([a-zA-Z0-9\\-]{36})\n" +
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

func TestTokenCloneCommand(t *testing.T) {
	t.Parallel()
	req := require.New(t)

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

	// Create a policy
	client := a.Client()

	_, _, err := client.ACL().PolicyCreate(
		&api.ACLPolicy{Name: "test-policy"},
		&api.WriteOptions{Token: "root"},
	)
	req.NoError(err)

	// create a token
	token, _, err := client.ACL().TokenCreate(
		&api.ACLToken{Description: "test", Policies: []*api.ACLTokenPolicyLink{&api.ACLTokenPolicyLink{Name: "test-policy"}}},
		&api.WriteOptions{Token: "root"},
	)
	req.NoError(err)

	// clone with description
	t.Run("Description", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
			"-description=test cloned",
		}

		code := cmd.Run(args)
		req.Empty(ui.ErrorWriter.String())
		req.Equal(code, 0)

		cloned := parseCloneOutput(t, ui.OutputWriter.String())

		req.Equal("test cloned", cloned.Description)
		req.Len(cloned.Policies, 1)

		apiToken, _, err := client.ACL().TokenRead(
			cloned.AccessorID,
			&api.QueryOptions{Token: "root"},
		)

		req.NoError(err)
		req.NotNil(apiToken)

		req.Equal(cloned.AccessorID, apiToken.AccessorID)
		req.Equal(cloned.SecretID, apiToken.SecretID)
		req.Equal(cloned.Description, apiToken.Description)
		req.Equal(cloned.Local, apiToken.Local)
		req.Equal(cloned.Policies, apiToken.Policies)
	})

	// clone without description
	t.Run("Without Description", func(t *testing.T) {
		ui := cli.NewMockUi()
		cmd := New(ui)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-id=" + token.AccessorID,
			"-token=root",
		}

		code := cmd.Run(args)
		req.Equal(code, 0)
		req.Empty(ui.ErrorWriter.String())

		cloned := parseCloneOutput(t, ui.OutputWriter.String())

		req.Equal("test", cloned.Description)
		req.Len(cloned.Policies, 1)

		apiToken, _, err := client.ACL().TokenRead(
			cloned.AccessorID,
			&api.QueryOptions{Token: "root"},
		)

		req.NoError(err)
		req.NotNil(apiToken)

		req.Equal(cloned.AccessorID, apiToken.AccessorID)
		req.Equal(cloned.SecretID, apiToken.SecretID)
		req.Equal(cloned.Description, apiToken.Description)
		req.Equal(cloned.Local, apiToken.Local)
		req.Equal(cloned.Policies, apiToken.Policies)
	})
}
