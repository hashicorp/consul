package agent

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/command/base"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/version"
	"github.com/mitchellh/cli"
)

func baseCommand(ui *cli.MockUi) base.Command {
	return base.Command{
		Flags: base.FlagSetNone,
		UI:    ui,
	}
}

func TestCommand_implements(t *testing.T) {
	t.Parallel()
	var _ cli.Command = new(Command)
}

func TestValidDatacenter(t *testing.T) {
	t.Parallel()
	shouldMatch := []string{
		"dc1",
		"east-aws-001",
		"PROD_aws01-small",
	}
	noMatch := []string{
		"east.aws",
		"east!aws",
		"first,second",
	}
	for _, m := range shouldMatch {
		if !validDatacenter.MatchString(m) {
			t.Fatalf("expected match: %s", m)
		}
	}
	for _, m := range noMatch {
		if validDatacenter.MatchString(m) {
			t.Fatalf("expected no match: %s", m)
		}
	}
}

// TestConfigFail should test command line flags that lead to an immediate error.
func TestConfigFail(t *testing.T) {
	t.Parallel()
	tests := []struct {
		args []string
		out  string
	}{
		{
			args: []string{"agent", "-server", "-data-dir", "foo", "-advertise", "0.0.0.0"},
			out:  "==> Advertise address cannot be 0.0.0.0\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", "foo", "-advertise", "::"},
			out:  "==> Advertise address cannot be ::\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", "foo", "-advertise", "[::]"},
			out:  "==> Advertise address cannot be [::]\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", "foo", "-advertise-wan", "0.0.0.0"},
			out:  "==> Advertise WAN address cannot be 0.0.0.0\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", "foo", "-advertise-wan", "::"},
			out:  "==> Advertise WAN address cannot be ::\n",
		},
		{
			args: []string{"agent", "-server", "-data-dir", "foo", "-advertise-wan", "[::]"},
			out:  "==> Advertise WAN address cannot be [::]\n",
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
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	cfg2 := TestConfig()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	cmd := &Command{
		Version:    version.Version,
		ShutdownCh: shutdownCh,
		Command:    baseCommand(new(cli.MockUi)),
	}

	serfAddr := fmt.Sprintf(
		"%s:%d",
		a.Config.BindAddr,
		a.Config.Ports.SerfLan)

	serfWanAddr := fmt.Sprintf(
		"%s:%d",
		a.Config.BindAddr,
		a.Config.Ports.SerfWan)

	args := []string{
		"-server",
		"-bind", a.Config.BindAddr,
		"-data-dir", tmpDir,
		"-node", fmt.Sprintf(`"%s"`, cfg2.NodeName),
		"-advertise", a.Config.BindAddr,
		"-retry-join", serfAddr,
		"-retry-interval", "1s",
		"-retry-join-wan", serfWanAddr,
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

func TestReadCliConfig(t *testing.T) {
	t.Parallel()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	// Test config parse
	{
		cmd := &Command{
			args: []string{
				"-data-dir", tmpDir,
				"-node", `"a"`,
				"-advertise-wan", "1.2.3.4",
				"-serf-wan-bind", "4.3.2.1",
				"-serf-lan-bind", "4.3.2.2",
				"-node-meta", "somekey:somevalue",
			},
			ShutdownCh: shutdownCh,
			Command:    baseCommand(new(cli.MockUi)),
		}

		config := cmd.readConfig()
		if config.AdvertiseAddrWan != "1.2.3.4" {
			t.Fatalf("expected -advertise-addr-wan 1.2.3.4 got %s", config.AdvertiseAddrWan)
		}
		if config.SerfWanBindAddr != "4.3.2.1" {
			t.Fatalf("expected -serf-wan-bind 4.3.2.1 got %s", config.SerfWanBindAddr)
		}
		if config.SerfLanBindAddr != "4.3.2.2" {
			t.Fatalf("expected -serf-lan-bind 4.3.2.2 got %s", config.SerfLanBindAddr)
		}
		if len(config.Meta) != 1 || config.Meta["somekey"] != "somevalue" {
			t.Fatalf("expected somekey=somevalue, got %v", config.Meta)
		}
	}

	// Test multiple node meta flags
	{
		cmd := &Command{
			args: []string{
				"-data-dir", tmpDir,
				"-node-meta", "somekey:somevalue",
				"-node-meta", "otherkey:othervalue",
			},
			ShutdownCh: shutdownCh,
			Command:    baseCommand(new(cli.MockUi)),
		}
		expected := map[string]string{
			"somekey":  "somevalue",
			"otherkey": "othervalue",
		}
		config := cmd.readConfig()
		if !reflect.DeepEqual(config.Meta, expected) {
			t.Fatalf("bad: %v %v", config.Meta, expected)
		}
	}

	// Test LeaveOnTerm and SkipLeaveOnInt defaults for server mode
	{
		ui := new(cli.MockUi)
		cmd := &Command{
			args: []string{
				"-node", `"server1"`,
				"-server",
				"-data-dir", tmpDir,
			},
			ShutdownCh: shutdownCh,
			Command:    baseCommand(ui),
		}

		config := cmd.readConfig()
		if config == nil {
			t.Fatalf(`Expected non-nil config object: %s`, ui.ErrorWriter.String())
		}
		if config.Server != true {
			t.Errorf(`Expected -server to be true`)
		}
		if (*config.LeaveOnTerm) != false {
			t.Errorf(`Expected LeaveOnTerm to be false in server mode`)
		}
		if (*config.SkipLeaveOnInt) != true {
			t.Errorf(`Expected SkipLeaveOnInt to be true in server mode`)
		}
	}

	// Test LeaveOnTerm and SkipLeaveOnInt defaults for client mode
	{
		ui := new(cli.MockUi)
		cmd := &Command{
			args: []string{
				"-data-dir", tmpDir,
				"-node", `"client"`,
			},
			ShutdownCh: shutdownCh,
			Command:    baseCommand(ui),
		}

		config := cmd.readConfig()
		if config == nil {
			t.Fatalf(`Expected non-nil config object: %s`, ui.ErrorWriter.String())
		}
		if config.Server != false {
			t.Errorf(`Expected server to be false`)
		}
		if (*config.LeaveOnTerm) != true {
			t.Errorf(`Expected LeaveOnTerm to be true in client mode`)
		}
		if *config.SkipLeaveOnInt != false {
			t.Errorf(`Expected SkipLeaveOnInt to be false in client mode`)
		}
	}

	// Test empty node name
	{
		cmd := &Command{
			args:       []string{"-node", `""`},
			ShutdownCh: shutdownCh,
			Command:    baseCommand(new(cli.MockUi)),
		}

		config := cmd.readConfig()
		if config != nil {
			t.Errorf(`Expected -node="" to fail`)
		}
	}
}

func TestRetryJoinFail(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	cmd := &Command{
		ShutdownCh: shutdownCh,
		Command:    baseCommand(new(cli.MockUi)),
	}

	serfAddr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Ports.SerfLan)

	args := []string{
		"-bind", cfg.BindAddr,
		"-data-dir", tmpDir,
		"-retry-join", serfAddr,
		"-retry-max", "1",
		"-retry-interval", "10ms",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestRetryJoinWanFail(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	tmpDir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	cmd := &Command{
		ShutdownCh: shutdownCh,
		Command:    baseCommand(new(cli.MockUi)),
	}

	serfAddr := fmt.Sprintf("%s:%d", cfg.BindAddr, cfg.Ports.SerfWan)

	args := []string{
		"-server",
		"-bind", cfg.BindAddr,
		"-data-dir", tmpDir,
		"-retry-join-wan", serfAddr,
		"-retry-max-wan", "1",
		"-retry-interval-wan", "10ms",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestDiscoverEC2Hosts(t *testing.T) {
	t.Parallel()
	if os.Getenv("AWS_REGION") == "" {
		t.Skip("AWS_REGION not set, skipping")
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("AWS_ACCESS_KEY_ID not set, skipping")
	}

	if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		t.Skip("AWS_SECRET_ACCESS_KEY not set, skipping")
	}

	c := &Config{
		RetryJoinEC2: RetryJoinEC2{
			Region:   os.Getenv("AWS_REGION"),
			TagKey:   "ConsulRole",
			TagValue: "Server",
		},
	}

	servers, err := c.discoverEc2Hosts(&log.Logger{})
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("bad: %v", servers)
	}
}

func TestDiscoverGCEHosts(t *testing.T) {
	t.Parallel()
	if os.Getenv("GCE_PROJECT") == "" {
		t.Skip("GCE_PROJECT not set, skipping")
	}

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" && os.Getenv("GCE_CONFIG_CREDENTIALS") == "" {
		t.Skip("GOOGLE_APPLICATION_CREDENTIALS or GCE_CONFIG_CREDENTIALS not set, skipping")
	}

	c := &Config{
		RetryJoinGCE: RetryJoinGCE{
			ProjectName:     os.Getenv("GCE_PROJECT"),
			ZonePattern:     os.Getenv("GCE_ZONE"),
			TagValue:        "consulrole-server",
			CredentialsFile: os.Getenv("GCE_CONFIG_CREDENTIALS"),
		},
	}

	servers, err := c.discoverGCEHosts(log.New(os.Stderr, "", log.LstdFlags))
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("bad: %v", servers)
	}
}

func TestDiscoverAzureHosts(t *testing.T) {
	subscriptionID := os.Getenv("ARM_SUBSCRIPTION_ID")
	tenantID := os.Getenv("ARM_TENANT_ID")
	clientID := os.Getenv("ARM_CLIENT_ID")
	clientSecret := os.Getenv("ARM_CLIENT_SECRET")
	environment := os.Getenv("ARM_ENVIRONMENT")

	if subscriptionID == "" || clientID == "" || clientSecret == "" || tenantID == "" {
		t.Skip("ARM_SUBSCRIPTION_ID, ARM_CLIENT_ID, ARM_CLIENT_SECRET and ARM_TENANT_ID " +
			"must be set to test Discover Azure Hosts")
	}

	if environment == "" {
		t.Log("Environments other than Public not supported at the moment")
	}

	c := &Config{
		RetryJoinAzure: RetryJoinAzure{
			SubscriptionID:  subscriptionID,
			ClientID:        clientID,
			SecretAccessKey: clientSecret,
			TenantID:        tenantID,
			TagName:         "type",
			TagValue:        "Foundation",
		},
	}

	servers, err := c.discoverAzureHosts(log.New(os.Stderr, "", log.LstdFlags))
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 3 {
		t.Fatalf("bad: %v", servers)
	}
}

func TestProtectDataDir(t *testing.T) {
	t.Parallel()
	dir := testutil.TempDir(t, "consul")
	defer os.RemoveAll(dir)

	if err := os.MkdirAll(filepath.Join(dir, "mdb"), 0700); err != nil {
		t.Fatalf("err: %v", err)
	}

	cfgFile := testutil.TempFile(t, "consul")
	defer os.Remove(cfgFile.Name())

	content := fmt.Sprintf(`{"server": true, "data_dir": "%s"}`, dir)
	_, err := cfgFile.Write([]byte(content))
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	ui := new(cli.MockUi)
	cmd := &Command{
		Command: baseCommand(ui),
		args:    []string{"-config-file=" + cfgFile.Name()},
	}
	if conf := cmd.readConfig(); conf != nil {
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

	ui := new(cli.MockUi)
	cmd := &Command{
		Command: baseCommand(ui),
		args:    []string{"-data-dir=" + dataDir, "-server=true"},
	}
	if conf := cmd.readConfig(); conf != nil {
		t.Fatalf("Should fail with bad data directory permissions")
	}
	if out := ui.ErrorWriter.String(); !strings.Contains(out, "Permission denied") {
		t.Fatalf("expected permission denied error, got: %s", out)
	}
}
