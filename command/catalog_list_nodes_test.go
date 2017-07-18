package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func testCatalogListNodesCommand(t *testing.T) (*cli.MockUi, *CatalogListNodesCommand) {
	ui := cli.NewMockUi()
	return ui, &CatalogListNodesCommand{
		BaseCommand: BaseCommand{
			Flags: FlagSetHTTP,
			UI:    ui,
		},
	}
}

func TestCatalogListNodesCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(CatalogListNodesCommand))
}

func TestCatalogListNodesCommand_Validation(t *testing.T) {
	t.Parallel()
	ui, c := testCatalogListNodesCommand(t)

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

func TestCatalogListNodesCommand_Run(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	t.Run("simple", func(t *testing.T) {
		ui, c := testCatalogListNodesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		for _, s := range []string{"Node", "ID", "Address", "DC"} {
			if !strings.Contains(output, s) {
				t.Errorf("expected %q to contain %q", output, s)
			}
		}
		for _, s := range []string{"TaggedAddresses", "Meta"} {
			if strings.Contains(output, s) {
				t.Errorf("expected %q to NOT contain %q", output, s)
			}
		}
	})

	t.Run("detailed", func(t *testing.T) {
		ui, c := testCatalogListNodesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-detailed",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		for _, s := range []string{"Node", "ID", "Address", "DC", "TaggedAddresses", "Meta"} {
			if !strings.Contains(output, s) {
				t.Errorf("expected %q to contain %q", output, s)
			}
		}
	})

	t.Run("node-meta", func(t *testing.T) {
		ui, c := testCatalogListNodesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-node-meta", "foo=bar",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.ErrorWriter.String()
		if expected := "No nodes match the given query"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("near", func(t *testing.T) {
		ui, c := testCatalogListNodesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-near", "_agent",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		if expected := "127.0.0.1"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("service_present", func(t *testing.T) {
		ui, c := testCatalogListNodesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service", "consul",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		if expected := "127.0.0.1"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("service_missing", func(t *testing.T) {
		ui, c := testCatalogListNodesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-service", "this-service-will-literally-never-exist",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.ErrorWriter.String()
		if expected := "No nodes match the given query"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})
}
