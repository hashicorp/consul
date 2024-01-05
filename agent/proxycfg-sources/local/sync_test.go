// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package local

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/local"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/token"
)

func TestSync(t *testing.T) {
	const (
		serviceID      = "some-service"
		serviceToken   = "some-service-token"
		otherServiceID = "other-service"
		userToken      = "user-token"
	)

	tokens := &token.Store{}
	tokens.UpdateUserToken(userToken, token.TokenSourceConfig)

	state := local.NewState(local.Config{}, hclog.NewNullLogger(), tokens)
	state.TriggerSyncChanges = func() {}

	state.AddServiceWithChecks(&structs.NodeService{
		ID:   serviceID,
		Kind: structs.ServiceKindConnectProxy,
	}, nil, serviceToken, false)

	cfgMgr := NewMockConfigManager(t)

	type registration struct {
		id      proxycfg.ProxyID
		service *structs.NodeService
		token   string
	}
	registerCh := make(chan registration)
	cfgMgr.On("Register", mock.Anything, mock.Anything, source, mock.Anything, true).
		Run(func(args mock.Arguments) {
			id := args.Get(0).(proxycfg.ProxyID)
			service := args.Get(1).(*structs.NodeService)
			token := args.Get(3).(string)
			registerCh <- registration{id, service, token}
		}).
		Return(nil)

	deregisterCh := make(chan proxycfg.ProxyID)
	cfgMgr.On("Deregister", mock.Anything, source).
		Run(func(args mock.Arguments) {
			id := args.Get(0).(proxycfg.ProxyID)
			deregisterCh <- id
		}).
		Return()

	cfgMgr.On("RegisteredProxies", source).
		Return([]proxycfg.ProxyID{{ServiceID: structs.ServiceID{ID: otherServiceID}}}).
		Once()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go Sync(ctx, SyncConfig{
		Manager: cfgMgr,
		State:   state,
		Tokens:  tokens,
		Logger:  hclog.NewNullLogger(),
	})

	// Expect the service in the local state to be registered.
	select {
	case reg := <-registerCh:
		require.Equal(t, serviceID, reg.service.ID)
		require.Equal(t, serviceToken, reg.token)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for service to be registered")
	}

	// Expect the service not in the local state to be de-registered.
	select {
	case id := <-deregisterCh:
		require.Equal(t, otherServiceID, id.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for service to be de-registered")
	}

	// Update the service (without a token) and expect it to be re-registered (with
	// the user token).
	cfgMgr.On("RegisteredProxies", source).
		Return([]proxycfg.ProxyID{}).
		Maybe()

	state.AddServiceWithChecks(&structs.NodeService{
		ID:   serviceID,
		Kind: structs.ServiceKindConnectProxy,
	}, nil, "", false)

	select {
	case reg := <-registerCh:
		require.Equal(t, serviceID, reg.service.ID)
		require.Equal(t, userToken, reg.token)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for service to be registered")
	}
}
