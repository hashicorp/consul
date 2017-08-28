package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

func testCatalogListServicesCommand(t *testing.T) (*cli.MockUi, *CatalogListServicesCommand) {
	ui := cli.NewMockUi()
	return ui, &CatalogListServicesCommand{
		BaseCommand: BaseCommand{
			Flags: FlagSetHTTP,
			UI:    ui,
		},
	}
}

func TestCatalogListServicesCommand_noTabs(t *testing.T) {
	t.Parallel()
	assertNoTabs(t, new(CatalogListServicesCommand))
}

func TestCatalogListServicesCommand_Validation(t *testing.T) {
	t.Parallel()
	ui, c := testCatalogListServicesCommand(t)

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

func TestCatalogListServicesCommand_Run(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	// Add another service with tags for testing
	if err := a.Client().Agent().ServiceRegister(&api.AgentServiceRegistration{
		Name:    "testing",
		Tags:    []string{"foo", "bar"},
		Port:    8080,
		Address: "127.0.0.1",
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("simple", func(t *testing.T) {
		ui, c := testCatalogListServicesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		if expected := "consul\ntesting\n"; output != expected {
			t.Errorf("expected %q to be %q", output, expected)
		}
	})

	t.Run("tags", func(t *testing.T) {
		ui, c := testCatalogListServicesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-tags",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		if expected := "bar,foo"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("node_missing", func(t *testing.T) {
		ui, c := testCatalogListServicesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-node", "not-a-real-node",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.ErrorWriter.String()
		if expected := "No services match the given query"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("node_present", func(t *testing.T) {
		ui, c := testCatalogListServicesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-node", a.Config.NodeName,
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.OutputWriter.String()
		if expected := "consul\ntesting\n"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("node-meta", func(t *testing.T) {
		ui, c := testCatalogListServicesCommand(t)
		args := []string{
			"-http-addr=" + a.HTTPAddr(),
			"-node-meta", "foo=bar",
		}
		code := c.Run(args)
		if code != 0 {
			t.Fatalf("bad exit code %d: %s", code, ui.ErrorWriter.String())
		}

		output := ui.ErrorWriter.String()
		if expected := "No services match the given query"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})
}
