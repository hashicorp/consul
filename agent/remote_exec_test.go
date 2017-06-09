package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-uuid"
)

func generateUUID() (ret string) {
	var err error
	if ret, err = uuid.GenerateUUID(); err != nil {
		panic(fmt.Sprintf("Unable to generate a UUID, %v", err))
	}
	return ret
}

func TestRexecWriter(t *testing.T) {
	t.Parallel()
	writer := &rexecWriter{
		BufCh:    make(chan []byte, 16),
		BufSize:  16,
		BufIdle:  10 * time.Millisecond,
		CancelCh: make(chan struct{}),
	}

	// Write short, wait for idle
	start := time.Now()
	n, err := writer.Write([]byte("test"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if n != 4 {
		t.Fatalf("bad: %v", n)
	}

	select {
	case b := <-writer.BufCh:
		if len(b) != 4 {
			t.Fatalf("Bad: %v", b)
		}
		if time.Now().Sub(start) < writer.BufIdle {
			t.Fatalf("too early")
		}
	case <-time.After(2 * writer.BufIdle):
		t.Fatalf("timeout")
	}

	// Write in succession to prevent the timeout
	writer.Write([]byte("test"))
	time.Sleep(writer.BufIdle / 2)
	writer.Write([]byte("test"))
	time.Sleep(writer.BufIdle / 2)
	start = time.Now()
	writer.Write([]byte("test"))

	select {
	case b := <-writer.BufCh:
		if len(b) != 12 {
			t.Fatalf("Bad: %v", b)
		}
		if time.Now().Sub(start) < writer.BufIdle {
			t.Fatalf("too early")
		}
	case <-time.After(2 * writer.BufIdle):
		t.Fatalf("timeout")
	}

	// Write large values, multiple flushes required
	writer.Write([]byte("01234567890123456789012345678901"))

	select {
	case b := <-writer.BufCh:
		if string(b) != "0123456789012345" {
			t.Fatalf("bad: %s", b)
		}
	default:
		t.Fatalf("should have buf")
	}
	select {
	case b := <-writer.BufCh:
		if string(b) != "6789012345678901" {
			t.Fatalf("bad: %s", b)
		}
	default:
		t.Fatalf("should have buf")
	}
}

func TestRemoteExecGetSpec(t *testing.T) {
	t.Parallel()
	testRemoteExecGetSpec(t, nil)
}

func TestRemoteExecGetSpec_ACLToken(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLDatacenter = "dc1"
	cfg.ACLToken = "root"
	cfg.ACLDefaultPolicy = "deny"
	testRemoteExecGetSpec(t, cfg)
}

func testRemoteExecGetSpec(t *testing.T, c *Config) {
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	event := &remoteExecEvent{
		Prefix:  "_rexec",
		Session: makeRexecSession(t, a.Agent),
	}
	defer destroySession(t, a.Agent, event.Session)

	spec := &remoteExecSpec{
		Command: "uptime",
		Script:  []byte("#!/bin/bash"),
		Wait:    time.Second,
	}
	buf, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	key := "_rexec/" + event.Session + "/job"
	setKV(t, a.Agent, key, buf)

	var out remoteExecSpec
	if !a.remoteExecGetSpec(event, &out) {
		t.Fatalf("bad")
	}
	if !reflect.DeepEqual(spec, &out) {
		t.Fatalf("bad spec")
	}
}

func TestRemoteExecWrites(t *testing.T) {
	t.Parallel()
	testRemoteExecWrites(t, nil)
}

func TestRemoteExecWrites_ACLToken(t *testing.T) {
	t.Parallel()
	cfg := TestConfig()
	cfg.ACLDatacenter = "dc1"
	cfg.ACLToken = "root"
	cfg.ACLDefaultPolicy = "deny"
	testRemoteExecWrites(t, cfg)
}

func testRemoteExecWrites(t *testing.T, c *Config) {
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	event := &remoteExecEvent{
		Prefix:  "_rexec",
		Session: makeRexecSession(t, a.Agent),
	}
	defer destroySession(t, a.Agent, event.Session)

	if !a.remoteExecWriteAck(event) {
		t.Fatalf("bad")
	}

	output := []byte("testing")
	if !a.remoteExecWriteOutput(event, 0, output) {
		t.Fatalf("bad")
	}
	if !a.remoteExecWriteOutput(event, 10, output) {
		t.Fatalf("bad")
	}

	exitCode := 1
	if !a.remoteExecWriteExitCode(event, &exitCode) {
		t.Fatalf("bad")
	}

	key := "_rexec/" + event.Session + "/" + a.Config.NodeName + "/ack"
	d := getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session {
		t.Fatalf("bad ack: %#v", d)
	}

	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/out/00000"
	d = getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session || !bytes.Equal(d.Value, output) {
		t.Fatalf("bad output: %#v", d)
	}

	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/out/0000a"
	d = getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session || !bytes.Equal(d.Value, output) {
		t.Fatalf("bad output: %#v", d)
	}

	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/exit"
	d = getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session || string(d.Value) != "1" {
		t.Fatalf("bad output: %#v", d)
	}
}

func testHandleRemoteExec(t *testing.T, command string, expectedSubstring string, expectedReturnCode string) {
	a := NewTestAgent(t.Name(), nil)
	defer a.Shutdown()

	event := &remoteExecEvent{
		Prefix:  "_rexec",
		Session: makeRexecSession(t, a.Agent),
	}
	defer destroySession(t, a.Agent, event.Session)

	spec := &remoteExecSpec{
		Command: command,
		Wait:    time.Second,
	}
	buf, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	key := "_rexec/" + event.Session + "/job"
	setKV(t, a.Agent, key, buf)

	buf, err = json.Marshal(event)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	msg := &UserEvent{
		ID:      generateUUID(),
		Payload: buf,
	}

	// Handle the event...
	a.handleRemoteExec(msg)

	// Verify we have an ack
	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/ack"
	d := getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session {
		t.Fatalf("bad ack: %#v", d)
	}

	// Verify we have output
	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/out/00000"
	d = getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session ||
		!bytes.Contains(d.Value, []byte(expectedSubstring)) {
		t.Fatalf("bad output: %#v", d)
	}

	// Verify we have an exit code
	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/exit"
	d = getKV(t, a.Agent, key)
	if d == nil || d.Session != event.Session || string(d.Value) != expectedReturnCode {
		t.Fatalf("bad output: %#v", d)
	}
}

func TestHandleRemoteExec(t *testing.T) {
	t.Parallel()
	testHandleRemoteExec(t, "uptime", "load", "0")
}

func TestHandleRemoteExecFailed(t *testing.T) {
	t.Parallel()
	testHandleRemoteExec(t, "echo failing;exit 2", "failing", "2")
}

func makeRexecSession(t *testing.T, a *Agent) string {
	args := structs.SessionRequest{
		Datacenter: a.config.Datacenter,
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node:      a.config.NodeName,
			LockDelay: 15 * time.Second,
		},
	}
	var out string
	if err := a.RPC("Session.Apply", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	return out
}

func destroySession(t *testing.T, a *Agent, session string) {
	args := structs.SessionRequest{
		Datacenter: a.config.Datacenter,
		Op:         structs.SessionDestroy,
		Session: structs.Session{
			ID: session,
		},
	}
	var out string
	if err := a.RPC("Session.Apply", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func setKV(t *testing.T, a *Agent, key string, val []byte) {
	write := structs.KVSRequest{
		Datacenter: a.config.Datacenter,
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:   key,
			Value: val,
		},
	}
	write.Token = a.config.ACLToken
	var success bool
	if err := a.RPC("KVS.Apply", &write, &success); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func getKV(t *testing.T, a *Agent, key string) *structs.DirEntry {
	req := structs.KeyRequest{
		Datacenter: a.config.Datacenter,
		Key:        key,
	}
	req.Token = a.config.ACLToken
	var out structs.IndexedDirEntries
	if err := a.RPC("KVS.Get", &req, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(out.Entries) > 0 {
		return out.Entries[0]
	}
	return nil
}
