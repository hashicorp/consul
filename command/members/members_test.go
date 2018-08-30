package members

import (
	"fmt"
	"strings"
	"testing"
	"reflect"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/agent"
	"github.com/mitchellh/cli"
)

func TestMembersCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestMembersCommand(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	args := []string{"-http-addr=" + a.HTTPAddr()}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Name
	if !strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	// Agent type
	if !strings.Contains(ui.OutputWriter.String(), "server") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	// Datacenter
	if !strings.Contains(ui.OutputWriter.String(), "dc1") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommand_WAN(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	args := []string{"-http-addr=" + a.HTTPAddr(), "-wan"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), fmt.Sprintf("%d", a.Config.SerfPortWAN)) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommand_statusFilter(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-status=a.*e",
	}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestMembersCommand_statusFilter_failed(t *testing.T) {
	t.Parallel()
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		"-status=(fail|left)",
	}

	code := c.Run(args)
	if code == 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if strings.Contains(ui.OutputWriter.String(), a.Config.NodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}

	if code != 2 {
		t.Fatalf("bad: %d", code)
	}
}

func TestMembersCommand_standardOutput(t *testing.T) {
	t.Parallel()

	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	member_agents := make([]*consulapi.AgentMember, 1)
	member_agents[0] = &consulapi.AgentMember{
		Name:   "first-node",
		Addr:   "2D33AE",
		Status: 1,
		Port:   2000,
		Tags: map[string]string{
			"segment": "dc1",
			"build":   "0.3",
			"vsn":     "2",
			"role":    "consul",
			"dc":      "dc1",
		},
	}

	actual := c.standardOutput(member_agents)
	expected := [] string{
		"Node|Address|Status|Type|Build|Protocol|DC|Segment",
		"first-node|:2000|alive|server|0.3|2|dc1|dc1"}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected \n%v to be \n%v", expected, actual)
	}
}
