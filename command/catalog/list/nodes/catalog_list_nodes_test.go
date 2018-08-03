package nodes

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testrpc"
	"github.com/mitchellh/cli"
)

func TestCatalogListNodesCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestCatalogListNodesCommand_Validation(t *testing.T) {
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

func TestCatalogListNodesCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	testrpc.WaitForLeader(t, a.RPC, "dc1")
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
		ui := cli.NewMockUi()
		c := New(ui)
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
		if expected := "No nodes match the given query"; !strings.Contains(output, expected) {
			t.Errorf("expected %q to contain %q", output, expected)
		}
	})

	t.Run("near", func(t *testing.T) {
		ui := cli.NewMockUi()
		c := New(ui)
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
		ui := cli.NewMockUi()
		c := New(ui)
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
		ui := cli.NewMockUi()
		c := New(ui)
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
