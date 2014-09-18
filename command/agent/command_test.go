package agent

import (
<<<<<<< HEAD
	"fmt"
	"io/ioutil"
	"log"
	"os"
=======
	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
	"io/ioutil"
	"path/filepath"
>>>>>>> command: test generated keyring file content and conflicting args for agent
	"testing"

	"github.com/hashicorp/consul/testutil"
	"github.com/mitchellh/cli"
)

func TestCommand_implements(t *testing.T) {
	var _ cli.Command = new(Command)
}

func TestValidDatacenter(t *testing.T) {
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

<<<<<<< HEAD
func TestRetryJoin(t *testing.T) {
	dir, agent := makeAgent(t, nextConfig())
	defer os.RemoveAll(dir)
	defer agent.Shutdown()

	conf2 := nextConfig()
	tmpDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	doneCh := make(chan struct{})
	shutdownCh := make(chan struct{})

	defer func() {
		close(shutdownCh)
		<-doneCh
	}()

	cmd := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	serfAddr := fmt.Sprintf(
		"%s:%d",
		agent.config.BindAddr,
		agent.config.Ports.SerfLan)

	serfWanAddr := fmt.Sprintf(
		"%s:%d",
		agent.config.BindAddr,
		agent.config.Ports.SerfWan)

	args := []string{
		"-server",
		"-data-dir", tmpDir,
		"-node", fmt.Sprintf(`"%s"`, conf2.NodeName),
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

	testutil.WaitForResult(func() (bool, error) {
		mem := agent.LANMembers()
		if len(mem) != 2 {
			return false, fmt.Errorf("bad: %#v", mem)
		}
		mem = agent.WANMembers()
		if len(mem) != 2 {
			return false, fmt.Errorf("bad (wan): %#v", mem)
		}
		return true, nil
	}, func(err error) {
		t.Fatalf(err.Error())
	})
}

func TestRetryJoinFail(t *testing.T) {
	conf := nextConfig()
	tmpDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	cmd := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	serfAddr := fmt.Sprintf("%s:%d", conf.BindAddr, conf.Ports.SerfLan)

	args := []string{
		"-data-dir", tmpDir,
		"-retry-join", serfAddr,
		"-retry-max", "1",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
	}
}

func TestRetryJoinWanFail(t *testing.T) {
	conf := nextConfig()
	tmpDir, err := ioutil.TempDir("", "consul")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	shutdownCh := make(chan struct{})
	defer close(shutdownCh)

	cmd := &Command{
		ShutdownCh: shutdownCh,
		Ui:         new(cli.MockUi),
	}

	serfAddr := fmt.Sprintf("%s:%d", conf.BindAddr, conf.Ports.SerfWan)

	args := []string{
		"-server",
		"-data-dir", tmpDir,
		"-retry-join-wan", serfAddr,
		"-retry-max-wan", "1",
	}

	if code := cmd.Run(args); code == 0 {
		t.Fatalf("bad: %d", code)
=======
func TestArgConflict(t *testing.T) {
	ui := new(cli.MockUi)
	c := &Command{Ui: ui}

	dir, err := ioutil.TempDir("", "agent")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	key := "HS5lJ+XuTlYKWaeGYyG+/A=="

	fileLAN := filepath.Join(dir, SerfLANKeyring)
	if err := testutil.InitKeyring(fileLAN, key); err != nil {
		t.Fatalf("err: %s", err)
	}

	args := []string{
		"-encrypt=" + key,
		"-data-dir=" + dir,
	}
	code := c.Run(args)
	if code != 1 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}
	if !strings.Contains(ui.ErrorWriter.String(), "keyring files exist") {
		t.Fatalf("bad: %#v", ui.ErrorWriter.String())
>>>>>>> command: test generated keyring file content and conflicting args for agent
	}
}
