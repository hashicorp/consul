package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/version"
	"github.com/mitchellh/cli"
)

// TestConfigFail should test command line flags that lead to an immediate error.
func TestConfigFail(t *testing.T) {
	t.Parallel()

	dataDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dataDir)

	tests := []struct {
		args []string
		out  string
	}{
		{
			args: []string{"agent", "-server", "-bind=10.0.0.1", "-datacenter="},
			out:  "==> datacenter cannot be empty\n",
		},
		{
			args: []string{"agent", "-server", "-bind=10.0.0.1"},
			out:  "==> data_dir cannot be empty\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", dataDir, "-advertise", "0.0.0.0", "-bind", "10.0.0.1"},
			out:  "==> Advertise address cannot be 0.0.0.0, :: or [::]\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", dataDir, "-advertise", "::", "-bind", "10.0.0.1"},
			out:  "==> Advertise address cannot be 0.0.0.0, :: or [::]\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", dataDir, "-advertise", "[::]", "-bind", "10.0.0.1"},
			out:  "==> Advertise address cannot be 0.0.0.0, :: or [::]\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", dataDir, "-advertise-wan", "0.0.0.0", "-bind", "10.0.0.1"},
			out:  "==> Advertise WAN address cannot be 0.0.0.0, :: or [::]\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", dataDir, "-advertise-wan", "::", "-bind", "10.0.0.1"},
			out:  "==> Advertise WAN address cannot be 0.0.0.0, :: or [::]\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", dataDir, "-advertise-wan", "[::]", "-bind", "10.0.0.1"},
			out:  "==> Advertise WAN address cannot be 0.0.0.0, :: or [::]\n",
		},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			cmd := exec.Command("consul", tt.args...)
			b, err := cmd.CombinedOutput()
			if got, want := err, "exit status 1"; got == nil || got.Error() != want {
				t.Fatalf("got err %q want %q", got, want)
			}
			if got, want := string(b), tt.out; got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

func TestRetryJoin(t *testing.T) {
	t.Parallel()
	t.Skip("fs: skipping tests that use cmd.Run until signal handling is fixed")
	a := agent.NewTestAgent(t.Name(), "")
	defer a.Shutdown()

	cfg2 := agent.TestConfig()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	ui := cli.NewMockUi()
	cmd := New(ui, "", version.Version, "", "", shutdownCh)

	args := []string{
		"-server",
		"-bind", a.Config.BindAddr.String(),
		"-data-dir", tmpDir,
		"-node", fmt.Sprintf(`"%s"`, cfg2.NodeName),
		"-advertise", a.Config.BindAddr.String(),
		"-retry-join", a.Config.SerfBindAddrLAN.String(),
		"-retry-interval", "1s",
		"-retry-join-wan", a.Config.SerfBindAddrWAN.String(),
		"-retry-interval-wan", "1s",
	}

	go func() {
		if code := cmd.Run(args); code != 0 {
			log.Printf("bad: %d", code)
		}
		close(doneCh)
	}()
	retry.Run(t, func(r *retry.R) {
		if got, want := len(a.LANMembers()), 2; got != want {
			r.Fatalf("got %d LAN members want %d", got, want)
		}
		if got, want := len(a.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members want %d", got, want)
		}
	})
}

func TestRetryJoinFail(t *testing.T) {
	t.Parallel()
	t.Skip("fs: skipping tests that use cmd.Run until signal handling is fixed")
	cfg := agent.TestConfig()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	ui := cli.NewMockUi()
	cmd := New(ui, "", "", "", "", shutdownCh)

	args := []string{
		"-bind", cfg.BindAddr.String(),
		"-data-dir", tmpDir,
		"-retry-join", cfg.SerfBindAddrLAN.String(),
		"-retry-max", "1",
		"-retry-interval", "10ms",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestRetryJoinWanFail(t *testing.T) {
	t.Parallel()
	t.Skip("fs: skipping tests that use cmd.Run until signal handling is fixed")
	cfg := agent.TestConfig()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	ui := cli.NewMockUi()
	cmd := New(ui, "", "", "", "", shutdownCh)

	args := []string{
		"-server",
		"-bind", cfg.BindAddr.String(),
		"-data-dir", tmpDir,
		"-retry-join-wan", cfg.SerfBindAddrWAN.String(),
		"-retry-max-wan", "1",
		"-retry-interval-wan", "10ms",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestProtectDataDir(t *testing.T) {
	t.Parallel()
	dir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dir)

	if err := os.MkdirAll(filepath.Join(dir, "mdb"), 0700); err != nil {
		t.Fatalf("err: %v", err)
	}

	cfgDir := testutil.TempDir(t, "consul-config")
	defer os.RemoveAll(cfgDir)

	cfgFilePath := filepath.Join(cfgDir, "consul.json")
	cfgFile, err := os.Create(cfgFilePath)
	if err != nil {
		t.Fatalf("Unable to create file %v, got error:%v", cfgFilePath, err)
	}

	content := fmt.Sprintf(`{"server": true, "bind_addr" : "10.0.0.1", "data_dir": "%s"}`, dir)
	_, err = cfgFile.Write([]byte(content))
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ui := cli.NewMockUi()
	cmd := New(ui, "", "", "", "", nil)
	args := []string{"-config-file=" + cfgFile.Name()}
	if code := cmd.Run(args); code == 0 {
		t.Fatalf("should fail")
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, dir) {
		t.Fatalf("expected mdb dir error, got: %s", out)
	}
}

func TestBadDataDirPermissions(t *testing.T) {
	t.Parallel()
	dir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dir)

	dataDir := filepath.Join(dir, "mdb")
	if err := os.MkdirAll(dataDir, 0400); err != nil {
		t.Fatalf("err: %v", err)
	}
	defer os.RemoveAll(dataDir)

	ui := cli.NewMockUi()
	cmd := New(ui, "", "", "", "", nil)
	args := []string{"-data-dir=" + dataDir, "-server=true", "-bind=10.0.0.1"}
	if code := cmd.Run(args); code == 0 {
		t.Fatalf("Should fail with bad data directory permissions")
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Permission denied") {
		t.Fatalf("expected permission denied error, got: %s", out)
	}
}
