package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func testCatalogListDatacentersCommand(t *testing.T) (*cli.MockUi, *CatalogListDatacentersCommand) {
	ui := cli.NewMockUi()
	return ui, &CatalogListDatacentersCommand{
		BaseCommand: BaseCommand{
			Flags: FlagSetHTTP,
			UI:    ui,
		},
	}
}

func TestCatalogListDatacentersCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(CatalogListDatacentersCommand))
}

func TestCatalogListDatacentersCommand_Validation(t *testing.T) {
	t.Parallel()
	ui, c := testCatalogListDatacentersCommand(t)

	cases := map[string]struct {
		args   []string
		output string
	}{
		"args": {
			[]string{"foo"},
			"Too many arguments",
		},
	}

	for name, tc := range cases {
		// Ensure our buffer is always clear
		if ui.ErrorWriter != nil {
			ui.ErrorWriter.Reset()
		}
		if ui.OutputWriter != nil {
			ui.OutputWriter.Reset()
		}

		code := c.Run(tc.args)
		if code == 0 {
			t.Errorf("%s: expected non-zero exit", name)
		}

		output := ui.ErrorWriter.String()
		if !strings.Contains(output, tc.output) {
			t.Errorf("%s: expected %q to contain %q", name, output, tc.output)
		}
	}
}

func TestCatalogListDatacentersCommand_Run(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	ui, c := testCatalogListDatacentersCommand(t)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	output := ui.OutputWriter.String()
	if !strings.Contains(output, "dc") {
		t.Errorf("bad: %#v", output)
	}
}
