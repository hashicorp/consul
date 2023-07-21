package services

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/testrpc"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/api"
	"github.com/mitchellh/cli"
)

func TestCatalogListServicesCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestCatalogListServicesCommand_Validation(t *testing.T) {
	t.Parallel()
	ui := cli.NewMockUi()
	c := New(ui)

	code := c.Run([]string{"foo"})
	if code == 0 {
		t.Fatal("expected non-zero exit")
	}
	if got, want := ui.ErrorWriter.String(), "Too many arguments"; !strings.Contains(got, want) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}

func TestCatalogListServicesCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

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
		ui := cli.NewMockUi()
		c := New(ui)
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
		ui := cli.NewMockUi()
		c := New(ui)
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
		ui := cli.NewMockUi()
		c := New(ui)
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
		ui := cli.NewMockUi()
		c := New(ui)
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
		ui := cli.NewMockUi()
		c := New(ui)
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
