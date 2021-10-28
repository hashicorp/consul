package members

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"testing"

	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
)

// TODO(partitions): split these tests

func TestMembersCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(cli.NewMockUi()).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestMembersCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, ``)
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

func TestMembersCommand_verticalBar(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()

	nodeName := "name|with|bars"
	a := agent.NewTestAgent(t, `node_name = "`+nodeName+`"`)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui)
	c.flags.SetOutput(ui.ErrorWriter)

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
	}

	code := c.Run(args)
	if code == 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	// Check for nodeName presense because it should not be parsed by columnize
	if !strings.Contains(ui.OutputWriter.String(), nodeName) {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestSortByMemberNamePartitionAndSegment(t *testing.T) {
	lib.SeedMathRand()

	// For the test data we'll give them names that would sort them backwards
	// if we only sorted by name.
	newData := func() []*consulapi.AgentMember {
		// NOTE: This should be sorted for assertions.
		return []*consulapi.AgentMember{
			// servers
			{Name: "p-betty", Tags: map[string]string{"role": "consul"}},
			{Name: "q-bob", Tags: map[string]string{"role": "consul"}},
			{Name: "r-bonnie", Tags: map[string]string{"role": "consul"}},
			// default clients
			{Name: "m-betty", Tags: map[string]string{}},
			{Name: "n-bob", Tags: map[string]string{}},
			{Name: "o-bonnie", Tags: map[string]string{}},
			// segment 1 clients
			{Name: "j-betty", Tags: map[string]string{"segment": "alpha"}},
			{Name: "k-bob", Tags: map[string]string{"segment": "alpha"}},
			{Name: "l-bonnie", Tags: map[string]string{"segment": "alpha"}},
			// segment 2 clients
			{Name: "g-betty", Tags: map[string]string{"segment": "beta"}},
			{Name: "h-bob", Tags: map[string]string{"segment": "beta"}},
			{Name: "i-bonnie", Tags: map[string]string{"segment": "beta"}},
			// partition 1 clients
			{Name: "d-betty", Tags: map[string]string{"ap": "part1"}},
			{Name: "e-bob", Tags: map[string]string{"ap": "part1"}},
			{Name: "f-bonnie", Tags: map[string]string{"ap": "part1"}},
			// partition 2 clients
			{Name: "a-betty", Tags: map[string]string{"ap": "part2"}},
			{Name: "b-bob", Tags: map[string]string{"ap": "part2"}},
			{Name: "c-bonnie", Tags: map[string]string{"ap": "part2"}},
		}
	}

	stringify := func(data []*consulapi.AgentMember) []string {
		var out []string
		for _, m := range data {
			out = append(out, fmt.Sprintf("<%s, %s, %s, %s>",
				m.Tags["role"],
				m.Tags["ap"],
				m.Tags["segment"],
				m.Name))
		}
		return out
	}

	expect := newData()
	for i := 0; i < 10; i++ {
		data := newData()
		rand.Shuffle(len(data), func(i, j int) {
			data[i], data[j] = data[j], data[i]
		})

		sort.Sort(ByMemberNamePartitionAndSegment(data))

		require.Equal(t, stringify(expect), stringify(data), "iteration #%d", i)
	}
}
