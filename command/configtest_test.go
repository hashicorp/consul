package command

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/mitchellh/cli"
)

func TestConfigTestCommand_implements(t *testing.T) {
	var _ cli.Command = &ConfigTestCommand{}
}

func TestConfigTestCommandFailOnEmptyFile(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpFile.Name())

	cmd := &ConfigTestCommand{
		Ui: new(cli.MockUi),
	}

	args := []string{
		"-config-file", tmpFile.Name(),
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnEmptyDir(t *testing.T) {
	td, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	cmd := &ConfigTestCommand{
		Ui: new(cli.MockUi),
	}

	args := []string{
		"-config-dir", td,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnMinimalConfigFile(t *testing.T) {
	td, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	fp := filepath.Join(td, "config.json")
	err = ioutil.WriteFile(fp, []byte(`{}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cmd := &ConfigTestCommand{
		Ui: new(cli.MockUi),
	}

	args := []string{
		"-config-file", fp,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnMinimalConfigDir(t *testing.T) {
	td, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	err = ioutil.WriteFile(filepath.Join(td, "config.json"), []byte(`{}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cmd := &ConfigTestCommand{
		Ui: new(cli.MockUi),
	}

	args := []string{
		"-config-dir", td,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestConfigTestCommandSucceedOnConfigDirWithEmptyFile(t *testing.T) {
	td, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(td)

	err = ioutil.WriteFile(filepath.Join(td, "config.json"), []byte{}, 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cmd := &ConfigTestCommand{
		Ui: new(cli.MockUi),
	}

	args := []string{
		"-config-dir", td,
	}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}
