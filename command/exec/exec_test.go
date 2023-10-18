// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package exec

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/consul/testrpc"

	"github.com/hashicorp/consul/agent"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/mitchellh/cli"
)

func TestExecCommand_noTabs(t *testing.T) {
	t.Parallel()
	if strings.ContainsRune(New(nil, nil).Help(), '\t') {
		t.Fatal("help has tabs")
	}
}

func TestExecCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-wait=1s", "uptime"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. Error:%#v  (std)Output:%#v", code, ui.ErrorWriter.String(), ui.OutputWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "load") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestExecCommand_NoShell(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a.HTTPAddr(), "-shell=false", "-wait=1s", "uptime"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. Error:%#v  (std)Output:%#v", code, ui.ErrorWriter.String(), ui.OutputWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "load") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestExecCommand_CrossDC(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a1 := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a1.Shutdown()

	testrpc.WaitForTestAgent(t, a1.RPC, "dc1")

	a2 := agent.NewTestAgent(t, `
		datacenter = "dc2"
		disable_remote_exec = false
	`)
	defer a2.Shutdown()

	testrpc.WaitForTestAgent(t, a2.RPC, "dc2")

	// Join over the WAN
	_, err := a2.JoinWAN([]string{a1.Config.SerfBindAddrWAN.String()})
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	retry.Run(t, func(r *retry.R) {
		if got, want := len(a1.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members on a1 want %d", got, want)
		}
		if got, want := len(a2.WANMembers()), 2; got != want {
			r.Fatalf("got %d WAN members on a2 want %d", got, want)
		}
	})

	ui := cli.NewMockUi()
	c := New(ui, nil)
	args := []string{"-http-addr=" + a1.HTTPAddr(), "-wait=500ms", "-datacenter=dc2", "uptime"}

	code := c.Run(args)
	if code != 0 {
		t.Fatalf("bad: %d. %#v", code, ui.ErrorWriter.String())
	}

	if !strings.Contains(ui.OutputWriter.String(), "load") {
		t.Fatalf("bad: %#v", ui.OutputWriter.String())
	}
}

func TestExecCommand_Validate(t *testing.T) {
	t.Parallel()
	conf := &rExecConf{}
	err := conf.validate()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	conf.node = "("
	err = conf.validate()
	if err == nil {
		t.Fatalf("err: %v", err)
	}

	conf.node = ""
	conf.service = "("
	err = conf.validate()
	if err == nil {
		t.Fatalf("err: %v", err)
	}

	conf.service = "()"
	conf.tag = "("
	err = conf.validate()
	if err == nil {
		t.Fatalf("err: %v", err)
	}

	conf.service = ""
	conf.tag = "foo"
	err = conf.validate()
	if err == nil {
		t.Fatalf("err: %v", err)
	}
}

func TestExecCommand_Sessions(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)
	c.apiclient = a.Client()
	id, err := c.createSession()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	se, _, err := a.Client().Session().Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if se == nil || se.Name != "Remote Exec" {
		t.Fatalf("bad: %v", se)
	}

	c.sessionID = id
	err = c.destroySession()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	se, _, err = a.Client().Session().Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if se != nil {
		t.Fatalf("bad: %v", se)
	}
}

func TestExecCommand_Sessions_Foreign(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a.Shutdown()

	ui := cli.NewMockUi()
	c := New(ui, nil)
	c.apiclient = a.Client()

	c.conf.foreignDC = true
	c.conf.localDC = "dc1"
	c.conf.localNode = "foo"

	var id string
	retry.Run(t, func(r *retry.R) {
		var err error
		id, err = c.createSession()
		if err != nil {
			r.Fatal(err)
		}
		if id == "" {
			r.Fatal("no id")
		}
	})

	se, _, err := a.Client().Session().Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if se == nil || se.Name != "Remote Exec via foo@dc1" {
		t.Fatalf("bad: %v", se)
	}

	c.sessionID = id
	err = c.destroySession()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	se, _, err = a.Client().Session().Info(id, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if se != nil {
		t.Fatalf("bad: %v", se)
	}
}

func TestExecCommand_UploadDestroy(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()

	c := New(ui, nil)
	c.apiclient = a.Client()
	id, err := c.createSession()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	c.sessionID = id

	c.conf.prefix = "_rexec"
	c.conf.cmd = "uptime"
	c.conf.wait = time.Second

	buf, err := c.makeRExecSpec()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	err = c.uploadPayload(buf)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	pair, _, err := a.Client().KV().Get("_rexec/"+id+"/job", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if pair == nil || len(pair.Value) == 0 {
		t.Fatalf("missing job spec")
	}

	err = c.destroyData()
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	pair, _, err = a.Client().KV().Get("_rexec/"+id+"/job", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	if pair != nil {
		t.Fatalf("should be destroyed")
	}
}

func TestExecCommand_StreamResults(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	a := agent.NewTestAgent(t, `
		disable_remote_exec = false
	`)
	defer a.Shutdown()

	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	ui := cli.NewMockUi()
	c := New(ui, nil)
	c.apiclient = a.Client()
	c.conf.prefix = "_rexec"

	id, err := c.createSession()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	c.sessionID = id

	ackCh := make(chan rExecAck, 128)
	heartCh := make(chan rExecHeart, 128)
	outputCh := make(chan rExecOutput, 128)
	exitCh := make(chan rExecExit, 128)
	doneCh := make(chan struct{})
	errCh := make(chan struct{}, 1)
	defer close(doneCh)
	go c.streamResults(doneCh, ackCh, heartCh, outputCh, exitCh, errCh)

	prefix := "_rexec/" + id + "/"
	ok, _, err := a.Client().KV().Acquire(&consulapi.KVPair{
		Key:     prefix + "foo/ack",
		Session: id,
	}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should be ok bro")
	}

	retry.Run(t, func(r *retry.R) {
		select {
		case a := <-ackCh:
			if a.Node != "foo" {
				r.Fatalf("bad: %#v", a)
			}
		case <-time.After(50 * time.Millisecond):
			r.Fatalf("timeout")
		}
	})

	ok, _, err = a.Client().KV().Acquire(&consulapi.KVPair{
		Key:     prefix + "foo/exit",
		Value:   []byte("127"),
		Session: id,
	}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should be ok bro")
	}

	retry.Run(t, func(r *retry.R) {
		select {
		case e := <-exitCh:
			if e.Node != "foo" || e.Code != 127 {
				r.Fatalf("bad: %#v", e)
			}
		case <-time.After(50 * time.Millisecond):
			r.Fatalf("timeout")
		}
	})

	// Random key, should ignore
	ok, _, err = a.Client().KV().Acquire(&consulapi.KVPair{
		Key:     prefix + "foo/random",
		Session: id,
	}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should be ok bro")
	}

	// Output heartbeat
	ok, _, err = a.Client().KV().Acquire(&consulapi.KVPair{
		Key:     prefix + "foo/out/00000",
		Session: id,
	}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should be ok bro")
	}

	retry.Run(t, func(r *retry.R) {
		select {
		case h := <-heartCh:
			if h.Node != "foo" {
				r.Fatalf("bad: %#v", h)
			}
		case <-time.After(50 * time.Millisecond):
			r.Fatalf("timeout")
		}
	})

	// Output value
	ok, _, err = a.Client().KV().Acquire(&consulapi.KVPair{
		Key:     prefix + "foo/out/00001",
		Value:   []byte("test"),
		Session: id,
	}, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !ok {
		t.Fatalf("should be ok bro")
	}

	retry.Run(t, func(r *retry.R) {
		select {
		case o := <-outputCh:
			if o.Node != "foo" || string(o.Output) != "test" {
				r.Fatalf("bad: %#v", o)
			}
		case <-time.After(50 * time.Millisecond):
			r.Fatalf("timeout")
		}
	})
}
