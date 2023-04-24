package rules

import (
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestRulesTranslateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRulesTranslateCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

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
	stdinR, stdinW := io.Pipe()

	ui := cli.NewMockUi()
	cmd := New(ui)
	cmd.testStdin = stdinR

	rules := "service \"\" { policy = \"write\" }"
	expected := "service_prefix \"\" {\n  policy = \"write\"\n}"

	// From a file
	t.Run("file", func(t *testing.T) {
		err := ioutil.WriteFile(testDir+"/rules.hcl", []byte(rules), 0644)
		require.NoError(t, err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"@" + testDir + "/rules.hcl",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), expected)
	})

	// From stdin
	t.Run("stdin", func(t *testing.T) {
		go func() {
			stdinW.Write([]byte(rules))
			stdinW.Close()
		}()

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-",
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), expected)
	})

	// From arg
	t.Run("arg", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			rules,
		}

		code := cmd.Run(args)
		require.Equal(t, code, 0)
		require.Empty(t, ui.ErrorWriter.String())
		require.Contains(t, ui.OutputWriter.String(), expected)
	})

	// cannot specify both secret and accessor
	t.Run("exclusive-options", func(t *testing.T) {
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"-token-secret",
			"-token-accessor",
			`token "" { policy = "write" }`,
		}

		code := cmd.Run(args)
		require.Equal(t, 1, code, 0)
		require.Equal(t, "Error - cannot specify both -token-secret and -token-accessor\n", ui.ErrorWriter.String())
	})
}
