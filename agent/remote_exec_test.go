package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

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

func TestRemoteExecHandler_GetExecSpec_Retries(t *testing.T) {
	event := remoteExecEvent{
		Datacenter: "the-dc",
		Prefix:     "_rexec",
		Session:    "the-session",
	}

	spec := &remoteExecSpec{
		Command: "uptime",
		Script:  []byte("#!/bin/bash"),
		Wait:    time.Second,
	}
	buf, err := json.Marshal(spec)
	require.NoError(t, err)

	fake := &fakeKV{
		getReturns: []structs.DirEntries{
			{},
			{{Value: buf}},
		},
	}
	e := &remoteExecHandler{
		Logger:       hclog.New(nil),
		KV:           fake,
		AgentTokener: new(token.Store),
	}

	var actual remoteExecSpec
	require.True(t, e.getExecSpec(event, &actual))
	require.Equal(t, spec, &actual)

	expectedReq := []structs.KeyRequest{
		{
			Datacenter: "the-dc",
			Key:        "_rexec/the-session/job",
			QueryOptions: structs.QueryOptions{
				AllowStale: true,
			},
		},
		{
			Datacenter: "the-dc",
			Key:        "_rexec/the-session/job",
			QueryOptions: structs.QueryOptions{
				AllowStale: false,
			},
		},
	}
	require.Equal(t, expectedReq, fake.getCalls)
}

type fakeKV struct {
	getReturns []structs.DirEntries
	getCalls   []structs.KeyRequest
	applyCalls []structs.KVSRequest
}

func (f *fakeKV) Get(_ context.Context, req structs.KeyRequest) (structs.IndexedDirEntries, error) {
	f.getCalls = append(f.getCalls, req)
	result := f.getReturns[0]
	if len(f.getReturns) > 1 {
		f.getReturns = f.getReturns[1:]
	}
	return structs.IndexedDirEntries{Entries: result}, nil
}

func (f *fakeKV) Apply(_ context.Context, req structs.KVSRequest) (bool, error) {
	f.applyCalls = append(f.applyCalls, req)
	return true, nil
}

func TestRemoteExecHandler_Writes(t *testing.T) {
	event := remoteExecEvent{
		Prefix:     "_rexec",
		Session:    "the-session",
		NodeName:   "node-name",
		Datacenter: "dc1",
	}

	fake := &fakeKV{}
	e := &remoteExecHandler{
		Logger:       hclog.New(nil),
		KV:           fake,
		AgentTokener: new(token.Store),
	}

	require.True(t, e.writeAck(event))

	output := []byte("testing")
	require.True(t, e.writeOutput(event, 0, output))
	require.True(t, e.writeOutput(event, 10, output))

	exitCode := 1
	require.True(t, e.writeExitCode(event, &exitCode))

	expected := []structs.KVSRequest{
		{
			Datacenter: "dc1",
			Op:         api.KVLock,
			DirEnt: structs.DirEntry{
				Key:     "_rexec/the-session/node-name/" + remoteExecAckSuffix,
				Session: "the-session",
			},
		},
		{
			Datacenter: "dc1",
			Op:         api.KVLock,
			DirEnt: structs.DirEntry{
				Key:     "_rexec/the-session/node-name/" + remoteExecOutputDivider + "/00000",
				Value:   output,
				Session: "the-session",
			},
		},
		{
			Datacenter: "dc1",
			Op:         api.KVLock,
			DirEnt: structs.DirEntry{
				Key:     "_rexec/the-session/node-name/" + remoteExecOutputDivider + "/0000a",
				Value:   output,
				Session: "the-session",
			},
		},
		{
			Datacenter: "dc1",
			Op:         api.KVLock,
			DirEnt: structs.DirEntry{
				Key:     "_rexec/the-session/node-name/" + remoteExecExitSuffix,
				Value:   []byte{'1'},
				Session: "the-session",
			},
		},
	}
	require.Equal(t, expected, fake.applyCalls)
}

func TestRemoteExecHandler_Handle_IntegrationWithAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()
	testHandleRemoteExec(t, "uptime", "load", "0")
}

func TestRemoteExecHandler_Handle_IntegrationWithAgent_ExecFailed(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}
	t.Parallel()
	testHandleRemoteExec(t, "echo failing;exit 2", "failing", "2")
}

func testHandleRemoteExec(t *testing.T, command string, expectedSubstring string, expectedReturnCode string) {
	a := NewTestAgent(t, "")
	defer a.Shutdown()
	testrpc.WaitForTestAgent(t, a.RPC, "dc1")

	handler := a.userEventHandler.deps.HandleRemoteExec

	retry.Run(t, func(r *retry.R) {
		event := remoteExecEvent{
			NodeName:   a.config.NodeName,
			Datacenter: a.config.Datacenter,
			Prefix:     "_rexec",
			Session:    makeRexecSession(t, a.Agent, ""),
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

		handler(event)

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
