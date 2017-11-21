package validate

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
)

func TestValidateCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestValidateCommand_FailOnEmptyFile(t *testing.T) {
	t.Parallel()
	tmpFile := testutil.TempFile(t, "consul")
	defer os.RemoveAll(tmpFile.Name())

	cmd := New(cli.NewMockUi())
	args := []string{tmpFile.Name()}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestValidateCommand_SucceedOnMinimalConfigFile(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	fp := filepath.Join(td, "config.json")
	err := ioutil.WriteFile(fp, []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cmd := New(cli.NewMockUi())
	args := []string{fp}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestValidateCommand_SucceedOnMinimalConfigDir(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	err := ioutil.WriteFile(filepath.Join(td, "config.json"), []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	cmd := New(cli.NewMockUi())
	args := []string{td}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestValidateCommand_Quiet(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")
	defer os.RemoveAll(td)

	fp := filepath.Join(td, "config.json")
	err := ioutil.WriteFile(fp, []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-quiet", td}

	if code := cmd.Run(args); code != 0 {
		t.Fatalf("bad: %d, %s", code, ui.ErrorWriter.String())
	}
	if ui.OutputWriter.String() != "" {
		t.Fatalf("bad: %v", ui.OutputWriter.String())
	}
}
