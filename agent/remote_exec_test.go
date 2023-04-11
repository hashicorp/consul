package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	// t.Parallel() // timing test. no parallel
	writer := &rexecWriter{
		BufCh:    make(chan []byte, 16),
		BufSize:  16,
		BufIdle:  100 * time.Millisecond,
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
		if time.Since(start) < writer.BufIdle {
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
		if time.Since(start) < writer.BufIdle {
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
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testRemoteExecGetSpec(t, "", "", true, "")
}

func TestRemoteExecGetSpec_ACLToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	testRemoteExecGetSpec(t, `
		primary_datacenter = "`+dc+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				default = "root"
			}
		}
	`, "root", true, dc)
}

func TestRemoteExecGetSpec_ACLAgentToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	testRemoteExecGetSpec(t, `
		primary_datacenter = "`+dc+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				agent = "root"
			}
		}
	`, "root", true, dc)
}

func TestRemoteExecGetSpec_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	testRemoteExecGetSpec(t, `
		primary_datacenter = "`+dc+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
			}
		}
	`, "root", false, dc)
}

func testRemoteExecGetSpec(t *testing.T, hcl string, token string, shouldSucceed bool, dc string) {
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	if dc != "" {
		testrpc.WaitForLeader(t, a.RPC, dc)
	} else {
		testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	}
	event := &remoteExecEvent{
		Prefix:  "_rexec",
		Session: makeRexecSession(t, a.Agent, token),
	}
	defer destroySession(t, a.Agent, event.Session, token)

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
	if err := setKV(a.Agent, key, buf, token); err != nil {
		t.Fatalf("err: %v", err)
	}

	var out remoteExecSpec
	if shouldSucceed != a.remoteExecGetSpec(event, &out) {
		t.Fatalf("bad")
	}
	if shouldSucceed && !reflect.DeepEqual(spec, &out) {
		t.Fatalf("bad spec")
	}
}

func TestRemoteExecWrites(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testRemoteExecWrites(t, "", "", true, "")
}

func TestRemoteExecWrites_ACLToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	testRemoteExecWrites(t, `
		primary_datacenter = "`+dc+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				default = "root"
			}
		}
	`, "root", true, dc)
}

func TestRemoteExecWrites_ACLAgentToken(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	testRemoteExecWrites(t, `
		primary_datacenter = "`+dc+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
				agent = "root"
			}
		}
	`, "root", true, dc)
}

func TestRemoteExecWrites_ACLDeny(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	dc := "dc1"
	testRemoteExecWrites(t, `
		primary_datacenter = "`+dc+`"

		acl {
			enabled = true
			default_policy = "deny"

			tokens {
				initial_management = "root"
			}
		}
	`, "root", false, dc)
}

func testRemoteExecWrites(t *testing.T, hcl string, token string, shouldSucceed bool, dc string) {
	a := NewTestAgent(t, hcl)
	defer a.Shutdown()
	if dc != "" {
		testrpc.WaitForLeader(t, a.RPC, dc)
	} else {
		// For slow machines, ensure we wait a bit
		testrpc.WaitForLeader(t, a.RPC, "dc1")
	}
	event := &remoteExecEvent{
		Prefix:  "_rexec",
		Session: makeRexecSession(t, a.Agent, token),
	}
	defer destroySession(t, a.Agent, event.Session, token)

	if shouldSucceed != a.remoteExecWriteAck(event) {
		t.Fatalf("bad")
	}

	output := []byte("testing")
	if shouldSucceed != a.remoteExecWriteOutput(event, 0, output) {
		t.Fatalf("bad")
	}
	if shouldSucceed != a.remoteExecWriteOutput(event, 10, output) {
		t.Fatalf("bad")
	}

	// Bypass the remaining checks if the write was expected to fail.
	if !shouldSucceed {
		return
	}

	exitCode := 1
	if !a.remoteExecWriteExitCode(event, &exitCode) {
		t.Fatalf("bad")
	}

	key := "_rexec/" + event.Session + "/" + a.Config.NodeName + "/ack"
	d, err := getKV(a.Agent, key, token)
	if d == nil || d.Session != event.Session || err != nil {
		t.Fatalf("bad ack: %#v", d)
	}

	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/out/00000"
	d, err = getKV(a.Agent, key, token)
	if d == nil || d.Session != event.Session || !bytes.Equal(d.Value, output) || err != nil {
		t.Fatalf("bad output: %#v", d)
	}

	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/out/0000a"
	d, err = getKV(a.Agent, key, token)
	if d == nil || d.Session != event.Session || !bytes.Equal(d.Value, output) || err != nil {
		t.Fatalf("bad output: %#v", d)
	}

	key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/exit"
	d, err = getKV(a.Agent, key, token)
	if d == nil || d.Session != event.Session || string(d.Value) != "1" || err != nil {
		t.Fatalf("bad output: %#v", d)
	}
}

func testHandleRemoteExec(t *testing.T, command string, expectedSubstring string, expectedReturnCode string) {
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")
	retry.Run(t, func(r *retry.R) {
		event := &remoteExecEvent{
			Prefix:  "_rexec",
			Session: makeRexecSession(t, a.Agent, ""),
		}
		defer destroySession(t, a.Agent, event.Session, "")

		spec := &remoteExecSpec{
			Command: command,
			Wait:    time.Second,
		}
		buf, err := json.Marshal(spec)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		key := "_rexec/" + event.Session + "/job"
		if err := setKV(a.Agent, key, buf, ""); err != nil {
			r.Fatalf("err: %v", err)
		}

		buf, err = json.Marshal(event)
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		msg := &UserEvent{
			ID:      generateUUID(),
			Payload: buf,
		}

		// Handle the event...
		a.handleRemoteExec(msg)

		// Verify we have an ack
		key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/ack"
		d, err := getKV(a.Agent, key, "")
		if d == nil || d.Session != event.Session || err != nil {
			r.Fatalf("bad ack: %#v", d)
		}

		// Verify we have output
		key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/out/00000"
		d, err = getKV(a.Agent, key, "")
		if d == nil || d.Session != event.Session ||
			!bytes.Contains(d.Value, []byte(expectedSubstring)) || err != nil {
			r.Fatalf("bad output: %#v", d)
		}

		// Verify we have an exit code
		key = "_rexec/" + event.Session + "/" + a.Config.NodeName + "/exit"
		d, err = getKV(a.Agent, key, "")
		if d == nil || d.Session != event.Session || string(d.Value) != expectedReturnCode || err != nil {
			r.Fatalf("bad output: %#v", d)
		}
	})
}

func TestHandleRemoteExec(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testHandleRemoteExec(t, "uptime", "load", "0")
}

func TestHandleRemoteExecFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	testHandleRemoteExec(t, "echo failing;exit 2", "failing", "2")
}

func makeRexecSession(t *testing.T, a *Agent, token string) string {
	args := structs.SessionRequest{
		Datacenter: a.config.Datacenter,
		Op:         structs.SessionCreate,
		Session: structs.Session{
			Node:      a.config.NodeName,
			LockDelay: 15 * time.Second,
		},
		WriteRequest: structs.WriteRequest{
			Token: token,
		},
	}
	var out string
	if err := a.RPC("Session.Apply", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
	return out
}

func destroySession(t *testing.T, a *Agent, session string, token string) {
	args := structs.SessionRequest{
		Datacenter: a.config.Datacenter,
		Op:         structs.SessionDestroy,
		Session: structs.Session{
			ID: session,
		},
		WriteRequest: structs.WriteRequest{
			Token: token,
		},
	}
	var out string
	if err := a.RPC("Session.Apply", &args, &out); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func setKV(a *Agent, key string, val []byte, token string) error {
	write := structs.KVSRequest{
		Datacenter: a.config.Datacenter,
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:   key,
			Value: val,
		},
		WriteRequest: structs.WriteRequest{
			Token: token,
		},
	}
	var success bool
	if err := a.RPC("KVS.Apply", &write, &success); err != nil {
		return err
	}
	return nil
}

func getKV(a *Agent, key string, token string) (*structs.DirEntry, error) {
	req := structs.KeyRequest{
		Datacenter: a.config.Datacenter,
		Key:        key,
		QueryOptions: structs.QueryOptions{
			Token: token,
		},
	}
	var out structs.IndexedDirEntries
	if err := a.RPC("KVS.Get", &req, &out); err != nil {
		return nil, err
	}
	if len(out.Entries) > 0 {
		return out.Entries[0], nil
	}
	return nil, nil
}
