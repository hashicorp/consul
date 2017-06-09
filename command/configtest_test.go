package command

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
)

func testConfigTestCommand(t *testing.T) (*cli.MockUi, *ConfigTestCommand) {
	ui := cli.NewMockUi()
	return ui, &ConfigTestCommand{
		BaseCommand: BaseCommand{
			UI:    ui,
			Flags: FlagSetNone,
		},
	}
}

func TestConfigTestCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = &ConfigTestCommand{}
}

func TestConfigTestCommandFailOnEmptyFile(t *testing.T) {
	t.Parallel()
	tmpFile := testutil.TempFile(t, "consul")
	defer os.RemoveAll(tmpFile.Name())

	_, cmd := testConfigTestCommand(t)

	args := []string{
		"-config-file", tmpFile.Name(),
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnEmptyDir(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	ui, cmd := testConfigTestCommand(t)

	args := []string{
		"-config-dir", td,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d, %s", code, ui.ErrorWriter.String())
	}
}

func TestConfigTestCommandSucceedOnMinimalConfigFile(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	fp := filepath.Join(td, "config.json")
	err := ioutil.WriteFile(fp, []byte(`{}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, cmd := testConfigTestCommand(t)

	args := []string{
		"-config-file", fp,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnMinimalConfigDir(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	err := ioutil.WriteFile(filepath.Join(td, "config.json"), []byte(`{}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, cmd := testConfigTestCommand(t)

	args := []string{
		"-config-dir", td,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnConfigDirWithEmptyFile(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	err := ioutil.WriteFile(filepath.Join(td, "config.json"), []byte{}, 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	_, cmd := testConfigTestCommand(t)

	args := []string{
		"-config-dir", td,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}
