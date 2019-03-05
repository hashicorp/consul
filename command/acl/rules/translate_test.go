package rules

import (
	"io"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/logger"
	"github.com/hashicorp/consul/testrpc"
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/assert"
)

func TestRulesTranslateCommand_noTabs(t *testing.T) {
	t.Parallel()

	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestRulesTranslateCommand(t *testing.T) {
	t.Parallel()
	assert := assert.New(t)

	testDir := testutil.TempDir(t, "acl")
	defer os.RemoveAll(testDir)

	a := agent.NewTestAgent(t.Name(), `
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
	stdinR, stdinW := io.Pipe()

	ui := cli.NewMockUi()
	cmd := New(ui)
	cmd.testStdin = stdinR

	rules := "service \"\" { policy = \"write\" }"
	expected := "service_prefix \"\" {\n  policy = \"write\"\n}"

	// From a file
	{
		err := ioutil.WriteFile(testDir+"/rules.hcl", []byte(rules), 0644)
		assert.NoError(err)

		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			"@" + testDir + "/rules.hcl",
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
		assert.Contains(ui.OutputWriter.String(), expected)
	}

	// From stdin
	{
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
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
		assert.Contains(ui.OutputWriter.String(), expected)
	}

	// From arg
	{
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-token=root",
			rules,
		}

		code := cmd.Run(args)
		assert.Equal(code, 0)
		assert.Empty(ui.ErrorWriter.String())
		assert.Contains(ui.OutputWriter.String(), expected)
	}
}
