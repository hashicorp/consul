package fsm

import (
	"bytes"
	"os"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/raft"
)

type MockSink struct {
	*bytes.Buffer
	cancel bool
}

func (m *MockSink) ID() string {
	return "Mock"
}

func (m *MockSink) Cancel() error {
	m.cancel = true
	return nil
}

func (m *MockSink) Close() error {
	return nil
}

func makeLog(buf []byte) *raft.Log {
	return &raft.Log{
		Index: 1,
		Term:  1,
		Type:  raft.LogCommand,
		Data:  buf,
	}
}

func TestFSM_IgnoreUnknown(t *testing.T) {
	t.Parallel()
	fsm, err := New(nil, os.Stderr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Create a new reap request
	type UnknownRequest struct {
		Foo string
	}
	req := UnknownRequest{Foo: "bar"}
	msgType := structs.IgnoreUnknownTypeFlag | 64
	buf, err := structs.Encode(msgType, req)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	// Apply should work, even though not supported
	resp := fsm.Apply(makeLog(buf))
	if err, ok := resp.(error); ok {
		t.Fatalf("resp: %v", err)
	}
}
