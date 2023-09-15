package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/mitchellh/cli"
	require "github.com/stretchr/testify/require"
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

	cmd := New(cli.NewMockUi())
	args := []string{tmpFile.Name()}

	code := cmd.Run(args)
	require.NotEqual(t, 0, code)
}

func TestValidateCommand_SucceedOnMinimalConfigFile(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	fp := filepath.Join(td, "config.json")
	err := os.WriteFile(fp, []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	require.Nilf(t, err, "err: %s", err)

	cmd := New(cli.NewMockUi())
	args := []string{fp}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
}

func TestValidateCommand_SucceedWithMinimalJSONConfigFormat(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	fp := filepath.Join(td, "json.conf")
	err := os.WriteFile(fp, []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	require.Nilf(t, err, "err: %s", err)

	cmd := New(cli.NewMockUi())
	args := []string{"--config-format", "json", fp}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
}

func TestValidateCommand_SucceedWithMinimalHCLConfigFormat(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	fp := filepath.Join(td, "hcl.conf")
	err := os.WriteFile(fp, []byte("bind_addr = \"10.0.0.1\"\ndata_dir = \""+td+"\""), 0644)
	require.Nilf(t, err, "err: %s", err)

	cmd := New(cli.NewMockUi())
	args := []string{"--config-format", "hcl", fp}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
}

func TestValidateCommand_SucceedWithJSONAsHCL(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	fp := filepath.Join(td, "json.conf")
	err := os.WriteFile(fp, []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	require.Nilf(t, err, "err: %s", err)

	cmd := New(cli.NewMockUi())
	args := []string{"--config-format", "hcl", fp}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
}

func TestValidateCommand_SucceedOnMinimalConfigDir(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	err := os.WriteFile(filepath.Join(td, "config.json"), []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	require.Nilf(t, err, "err: %s", err)

	cmd := New(cli.NewMockUi())
	args := []string{td}

	code := cmd.Run(args)
	require.Equal(t, 0, code)
}

func TestValidateCommand_FailForInvalidJSONConfigFormat(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	fp := filepath.Join(td, "hcl.conf")
	err := os.WriteFile(fp, []byte(`bind_addr = "10.0.0.1"\ndata_dir = "`+td+`"`), 0644)
	require.Nilf(t, err, "err: %s", err)

	cmd := New(cli.NewMockUi())
	args := []string{"--config-format", "json", fp}

	code := cmd.Run(args)
	require.NotEqual(t, 0, code)
}

func TestValidateCommand_Quiet(t *testing.T) {
	t.Parallel()
	td := testutil.TempDir(t, "consul")

	fp := filepath.Join(td, "config.json")
	err := os.WriteFile(fp, []byte(`{"bind_addr":"10.0.0.1", "data_dir":"`+td+`"}`), 0644)
	require.Nilf(t, err, "err: %s", err)

	ui := cli.NewMockUi()
	cmd := New(ui)
	args := []string{"-quiet", td}

	code := cmd.Run(args)
	require.Equalf(t, 0, code, "return code - expected: 0, bad: %d, %s", code, ui.ErrorWriter.String())
	require.Equal(t, "", ui.OutputWriter.String())
}
