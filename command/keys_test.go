package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/agent"
	"github.com/mitchellh/cli"
)

func TestKeysCommand_implements(t *testing.T) {
	var _ cli.Command = &KeysCommand{}
}

func TestKeysCommand_list(t *testing.T) {
	conf := agent.Config{EncryptKey: "HS5lJ+XuTlYKWaeGYyG+/A=="}

	a1 := testAgentWithConfig(&conf, t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &KeysCommand{Ui: ui}
	args := []string{"-list", "-rpc-addr=" + a1.addr}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), conf.EncryptKey) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}
