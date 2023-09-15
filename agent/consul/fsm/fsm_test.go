// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fsm

import (
	"bytes"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
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
	logger := testutil.Logger(t)
	fsm, err := New(nil, logger)
	assert.Nil(t, err)

	// Create a new reap request
	type UnknownRequest struct {
		Foo string
	}
	req := UnknownRequest{Foo: "bar"}
	msgType := structs.IgnoreUnknownTypeFlag | 75
	buf, err := structs.Encode(msgType, req)
	assert.Nil(t, err)

	// Apply should work, even though not supported
	resp := fsm.Apply(makeLog(buf))
	err, ok := resp.(error)
	assert.False(t, ok, "response: %s", err)
}

func TestFSM_NilLogger(t *testing.T) {
	fsm, err := New(nil, nil)
	assert.Nil(t, err)
	assert.NotNil(t, fsm)
}
