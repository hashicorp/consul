package register

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func TestCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestCommand_File(t *testing.T) {
	t.Parallel()

	require := require.New(t)
	a := agent.NewTestAgent(t.Name(), ``)
	defer a.Shutdown()
	client := a.Client()

	ui := cli.NewMockUi()
	c := New(ui)

	contents := `{ "Service": { "Name": "web" } }`
	f := testFile(t, "json")
	defer os.Remove(f.Name())
	if _, err := f.WriteString(contents); err != nil {
		t.Fatalf("err: %#v", err)
	}

	args := []string{
		"-http-addr=" + a.HTTPAddr(),
		f.Name(),
	}

	require.Equal(0, c.Run(args), ui.ErrorWriter.String())

	svcs, err := client.Agent().Services()
	require.NoError(err)
	require.Len(svcs, 1)

	svc := svcs["web"]
	require.NotNil(svc)
}

func testFile(t *testing.T, suffix string) *os.File {
	f := testutil.TempFile(t, "register-test-file")
	if err := f.Close(); err != nil {
		t.Fatalf("err: %s", err)
	}

	newName := f.Name() + "." + suffix
	if err := os.Rename(f.Name(), newName); err != nil {
		os.Remove(f.Name())
		t.Fatalf("err: %s", err)
	}

	f, err := os.Create(newName)
	if err != nil {
		os.Remove(newName)
		t.Fatalf("err: %s", err)
	}

	return f
}
