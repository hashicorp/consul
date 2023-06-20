// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"context"
	"io"

	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/mock"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/acl/resolver"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

type delegateMock struct {
	mock.Mock
}

func (m *delegateMock) GetLANCoordinate() (lib.CoordinateSet, error) {
	ret := m.Called()
	return ret.Get(0).(lib.CoordinateSet), ret.Error(1)
}

func (m *delegateMock) Leave() error {
	return m.Called().Error(0)
}

func (m *delegateMock) LANMembersInAgentPartition() []serf.Member {
	return m.Called().Get(0).([]serf.Member)
}

func (m *delegateMock) LANMembers(f consul.LANMemberFilter) ([]serf.Member, error) {
	ret := m.Called(f)
	return ret.Get(0).([]serf.Member), ret.Error(1)
}

func (m *delegateMock) AgentLocalMember() serf.Member {
	return m.Called().Get(0).(serf.Member)
}

func (m *delegateMock) JoinLAN(addrs []string, entMeta *acl.EnterpriseMeta) (n int, err error) {
	ret := m.Called(addrs, entMeta)
	return ret.Int(0), ret.Error(1)
}

func (m *delegateMock) RemoveFailedNode(node string, prune bool, entMeta *acl.EnterpriseMeta) error {
	return m.Called(node, prune, entMeta).Error(0)
}

func (m *delegateMock) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzContext *acl.AuthorizerContext) (resolver.Result, error) {
	ret := m.Called(token, entMeta, authzContext)
	return ret.Get(0).(resolver.Result), ret.Error(1)
}

func (m *delegateMock) RPC(ctx context.Context, method string, args interface{}, reply interface{}) error {
	return m.Called(method, args, reply).Error(0)
}

func (m *delegateMock) SnapshotRPC(args *structs.SnapshotRequest, in io.Reader, out io.Writer, replyFn structs.SnapshotReplyFn) error {
	return m.Called(args, in, out, replyFn).Error(0)
}

func (m *delegateMock) Shutdown() error {
	return m.Called().Error(0)
}

func (m *delegateMock) Stats() map[string]map[string]string {
	return m.Called().Get(0).(map[string]map[string]string)
}

func (m *delegateMock) ReloadConfig(config consul.ReloadableConfig) error {
	return m.Called(config).Error(0)
}
