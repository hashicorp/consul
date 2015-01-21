package command

import (
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/mitchellh/cli"
)

func TestMaintCommand_implements(t *testing.T) {
	var _ cli.Command = &MaintCommand{}
}

func TestMaintCommandRun_NoArgs(t *testing.T) {
	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	if code := c.Run([]string{}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if strings.TrimSpace(ui.ErrorWriter.String()) != c.Help() {
		t.Fatalf("bad:\n%s", ui.ErrorWriter.String())
	}
}

func TestMaintCommandRun_ConflictingArgs(t *testing.T) {
	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	if code := c.Run([]string{"-enable", "-disable"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}

	if code := c.Run([]string{"-disable", "-reason=broken"}); code != 1 {
		t.Fatalf("expected return code 1, got %d", code)
	}
}

func TestMaintCommandRun_EnableNodeMaintenance(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	args := []string{
		"-http-addr=" + a1.httpAddr,
		"-enable",
		"-reason=broken",
	}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "now enabled") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMaintCommandRun_DisableNodeMaintenance(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	args := []string{
		"-http-addr=" + a1.httpAddr,
		"-disable",
	}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "now disabled") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMaintCommandRun_EnableServiceMaintenance(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	// Register the service
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a1.agent.AddService(service, nil, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	args := []string{
		"-http-addr=" + a1.httpAddr,
		"-enable",
		"-service=test",
		"-reason=broken",
	}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "now enabled") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMaintCommandRun_DisableServiceMaintenance(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	// Register the service
	service := &structs.NodeService{
		ID:      "test",
		Service: "test",
	}
	if err := a1.agent.AddService(service, nil, false); err != nil {
		t.Fatalf("err: %v", err)
	}

	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	args := []string{
		"-http-addr=" + a1.httpAddr,
		"-disable",
		"-service=test",
	}
	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "now disabled") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMaintCommandRun_ServiceMaintenance_NoService(t *testing.T) {
	a1 := testAgent(t)
	defer a1.Shutdown()

	ui := new(cli.MockUi)
	c := &MaintCommand{Ui: ui}

	args := []string{
		"-http-addr=" + a1.httpAddr,
		"-enable",
		"-service=redis",
		"-reason=broken",
	}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("expected response code 1, got %d", code)
	}

	if !strings.Contains(ui.ErrorWriter.String(), "No service registered") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
	}
}
