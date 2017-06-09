package command

import (
	"encoding/base64"
	"testing"

	"github.com/mitchellh/cli"
)

func TestKeygenCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &KeygenCommand{}
}

func TestKeygenCommand(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := &KeygenCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetNone,
		},
	}
	code := c.Run(nil)
	if code != 0 {
		t.Fatalf("bad: %d", code)
	}

	output := ui.OutputWriter.String()
	result, err := base64.StdEncoding.DecodeString(output)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	if len(result) != 16 {
		t.Fatalf("bad: %#v", result)
	}
}
