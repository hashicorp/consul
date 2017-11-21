package imp

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestKVImportCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestKVImportCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	const json = `[
		{
			"key": "foo",
			"flags": 0,
			"value": "YmFyCg=="
		},
		{
			"key": "foo/a",
			"flags": 0,
			"value": "YmF6Cg=="
		}
	]`

	ui := cli.NewMockUi()
	c := New(ui)
	c.testStdin = strings.NewReader(json)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	pair, _, err := client.KV().Get("foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(pair.Value)) != "bar" {
		t.Fatalf("bad: expected: bar, got %s", pair.Value)
	}

	pair, _, err = client.KV().Get("foo/a", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(pair.Value)) != "baz" {
		t.Fatalf("bad: expected: baz, got %s", pair.Value)
	}
}
