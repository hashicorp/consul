// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package proxytracker

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/mesh"
	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestProxyTracker_Watch(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
	proxyReferenceKey := resource.NewReferenceKey(resourceID)
	lim := NewMockSessionLimiter(t)
	session1 := newMockSession(t)
	session1TermCh := make(limiter.SessionTerminatedChan)
	session1.On("Terminated").Return(session1TermCh)
	session1.On("End").Return()
	lim.On("BeginSession").Return(session1, nil)
	logger := testutil.Logger(t)

	pt := NewProxyTracker(ProxyTrackerConfig{
		Logger:         logger,
		SessionLimiter: lim,
	})

	// Watch()
	proxyStateChan, _, cancelFunc, err := pt.Watch(resourceID, "node 1", "token")
	require.NoError(t, err)

	// ensure New Proxy Connection message is sent
	newProxyMsg := <-pt.EventChannel()
	require.Equal(t, resourceID.Name, newProxyMsg.Obj.Key())

	// watchData is stored in the proxies array with a nil state
	watchData, ok := pt.proxies[proxyReferenceKey]
	require.True(t, ok)
	require.NotNil(t, watchData)
	require.Nil(t, watchData.state)

	// calling cancelFunc does the following:
	// - closes the proxy state channel
	// - and removes the map entry for the proxy
	// - session is ended
	cancelFunc()

	// read channel to see if there is data and it is open.
	receivedState, channelOpen := <-proxyStateChan
	require.Nil(t, receivedState)
	require.False(t, channelOpen)

	// key is removed from proxies array
	_, ok = pt.proxies[proxyReferenceKey]
	require.False(t, ok)

	// session ended
	session1.AssertCalled(t, "Terminated")
	session1.AssertCalled(t, "End")
}

func TestProxyTracker_Watch_ErrorConsumerNotReady(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
	proxyReferenceKey := resource.NewReferenceKey(resourceID)
	lim := NewMockSessionLimiter(t)
	session1 := newMockSession(t)
	session1.On("End").Return()
	lim.On("BeginSession").Return(session1, nil)
	logger := testutil.Logger(t)

	pt := NewProxyTracker(ProxyTrackerConfig{
		Logger:         logger,
		SessionLimiter: lim,
	})

	//fill up buffered channel while the consumer is not ready to simulate the error
	for i := 0; i < 1000; i++ {
		event := controller.Event{Obj: &ProxyConnection{ProxyID: resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, fmt.Sprintf("test%d", i), registry).ID()}}
		pt.newProxyConnectionCh <- event
	}

	// Watch()
	proxyStateChan, sessionTerminatedCh, cancelFunc, err := pt.Watch(resourceID, "node 1", "token")
	require.Nil(t, cancelFunc)
	require.Nil(t, proxyStateChan)
	require.Nil(t, sessionTerminatedCh)
	require.Error(t, err)
	require.Equal(t, "failed to notify the controller of the proxy connecting", err.Error())

	// it is not stored in the proxies array
	watchData, ok := pt.proxies[proxyReferenceKey]
	require.False(t, ok)
	require.Nil(t, watchData)
}

func TestProxyTracker_Watch_ArgValidationErrors(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	type testcase struct {
		description   string
		proxyID       *pbresource.ID
		nodeName      string
		token         string
		expectedError error
	}
	testcases := []*testcase{
		{
			description:   "Empty proxyID",
			proxyID:       nil,
			nodeName:      "something",
			token:         "something",
			expectedError: errors.New("proxyID is required"),
		},
		{
			description:   "Empty nodeName",
			proxyID:       resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID(),
			nodeName:      "",
			token:         "something",
			expectedError: errors.New("nodeName is required"),
		},
		{
			description:   "resource is not ProxyStateTemplate",
			proxyID:       resourcetest.Resource(mesh.ProxyConfigurationType, "test", registry).ID(),
			nodeName:      "something",
			token:         "something else",
			expectedError: errors.New("proxyID must be a ProxyStateTemplate"),
		},
	}
	for _, tc := range testcases {
		lim := NewMockSessionLimiter(t)
		lim.On("BeginSession").Return(nil, nil).Maybe()
		logger := testutil.Logger(t)

		pt := NewProxyTracker(ProxyTrackerConfig{
			Logger:         logger,
			SessionLimiter: lim,
		})

		// Watch()
		proxyStateChan, sessionTerminateCh, cancelFunc, err := pt.Watch(tc.proxyID, tc.nodeName, tc.token)
		require.Error(t, err)
		require.Equal(t, tc.expectedError, err)
		require.Nil(t, proxyStateChan)
		require.Nil(t, sessionTerminateCh)
		require.Nil(t, cancelFunc)
	}
}

func TestProxyTracker_Watch_SessionLimiterError(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
	lim := NewMockSessionLimiter(t)
	lim.On("BeginSession").Return(nil, errors.New("kaboom"))
	logger := testutil.Logger(t)
	pt := NewProxyTracker(ProxyTrackerConfig{
		Logger:         logger,
		SessionLimiter: lim,
	})

	// Watch()
	proxyStateChan, sessionTerminateCh, cancelFunc, err := pt.Watch(resourceID, "node 1", "token")
	require.Error(t, err)
	require.Equal(t, "kaboom", err.Error())
	require.Nil(t, proxyStateChan)
	require.Nil(t, sessionTerminateCh)
	require.Nil(t, cancelFunc)
}

func TestProxyTracker_PushChange(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
	proxyReferenceKey := resource.NewReferenceKey(resourceID)
	lim := NewMockSessionLimiter(t)
	session1 := newMockSession(t)
	session1TermCh := make(limiter.SessionTerminatedChan)
	session1.On("Terminated").Return(session1TermCh)
	lim.On("BeginSession").Return(session1, nil)
	logger := testutil.Logger(t)

	pt := NewProxyTracker(ProxyTrackerConfig{
		Logger:         logger,
		SessionLimiter: lim,
	})

	// Watch()
	proxyStateChan, _, _, err := pt.Watch(resourceID, "node 1", "token")
	require.NoError(t, err)

	// PushChange
	proxyState := &mesh.ProxyState{ProxyState: &pbmesh.ProxyState{
		IntentionDefaultAllow: true,
	}}

	// using a goroutine so that the channel and main test thread do not cause
	// blocking issues with each other
	go func() {
		err = pt.PushChange(resourceID, proxyState)
		require.NoError(t, err)
	}()

	// channel receives a copy
	receivedState, channelOpen := <-proxyStateChan
	require.True(t, channelOpen)
	require.Equal(t, proxyState, receivedState)

	// it is stored in the proxies array
	watchData, ok := pt.proxies[proxyReferenceKey]
	require.True(t, ok)
	require.Equal(t, proxyState, watchData.state)
}

func TestProxyTracker_PushChanges_ErrorProxyNotConnected(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
	lim := NewMockSessionLimiter(t)
	logger := testutil.Logger(t)

	pt := NewProxyTracker(ProxyTrackerConfig{
		Logger:         logger,
		SessionLimiter: lim,
	})

	// PushChange
	proxyState := &mesh.ProxyState{ProxyState: &pbmesh.ProxyState{
		IntentionDefaultAllow: true,
	}}

	err := pt.PushChange(resourceID, proxyState)
	require.Error(t, err)
	require.Equal(t, "proxyState change could not be sent because proxy is not connected", err.Error())
}

func TestProxyTracker_ProxyConnectedToServer(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	type testcase struct {
		name              string
		shouldExist       bool
		preProcessingFunc func(pt *ProxyTracker, resourceID *pbresource.ID, limiter *MockSessionLimiter, session *mockSession, channel limiter.SessionTerminatedChan)
	}
	testsCases := []*testcase{
		{
			name:        "Resource that has not been sent through Watch() should return false",
			shouldExist: false,
			preProcessingFunc: func(pt *ProxyTracker, resourceID *pbresource.ID, limiter *MockSessionLimiter, session *mockSession, channel limiter.SessionTerminatedChan) {
				session.On("Terminated").Return(channel).Maybe()
				session.On("End").Return().Maybe()
				limiter.On("BeginSession").Return(session, nil).Maybe()
			},
		},
		{
			name:        "Resource used that is already passed in through Watch() should return true",
			shouldExist: true,
			preProcessingFunc: func(pt *ProxyTracker, resourceID *pbresource.ID, limiter *MockSessionLimiter, session *mockSession, channel limiter.SessionTerminatedChan) {
				session.On("Terminated").Return(channel).Maybe()
				session.On("End").Return().Maybe()
				limiter.On("BeginSession").Return(session, nil)
				_, _, _, _ = pt.Watch(resourceID, "node 1", "token")
			},
		},
	}

	for _, tc := range testsCases {
		lim := NewMockSessionLimiter(t)
		session1 := newMockSession(t)
		session1TermCh := make(limiter.SessionTerminatedChan)
		logger := testutil.Logger(t)

		pt := NewProxyTracker(ProxyTrackerConfig{
			Logger:         logger,
			SessionLimiter: lim,
		})
		resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
		tc.preProcessingFunc(pt, resourceID, lim, session1, session1TermCh)
		require.Equal(t, tc.shouldExist, pt.ProxyConnectedToServer(resourceID))
	}
}

func TestProxyTracker_Shutdown(t *testing.T) {
	registry := resource.NewRegistry()
	mesh.RegisterTypes(registry)

	resourceID := resourcetest.Resource(mesh.ProxyStateTemplateConfigurationType, "test", registry).ID()
	proxyReferenceKey := resource.NewReferenceKey(resourceID)
	lim := NewMockSessionLimiter(t)
	session1 := newMockSession(t)
	session1TermCh := make(limiter.SessionTerminatedChan)
	session1.On("Terminated").Return(session1TermCh)
	session1.On("End").Return().Maybe()
	lim.On("BeginSession").Return(session1, nil)
	logger := testutil.Logger(t)

	pt := NewProxyTracker(ProxyTrackerConfig{
		Logger:         logger,
		SessionLimiter: lim,
	})

	// Watch()
	proxyStateChan, _, _, err := pt.Watch(resourceID, "node 1", "token")
	require.NoError(t, err)

	pt.Shutdown()

	// proxy channels are all disconnected and proxy is removed from proxies map
	receivedState, channelOpen := <-proxyStateChan
	require.Nil(t, receivedState)
	require.False(t, channelOpen)
	_, ok := pt.proxies[proxyReferenceKey]
	require.False(t, ok)

	// shutdownCh is closed
	select {
	case <-pt.ShutdownChannel():
	default:
		t.Fatalf("shutdown channel should be closed")
	}
	// newProxyConnectionCh is closed
	select {
	case <-pt.EventChannel():
	default:
		t.Fatalf("shutdown channel should be closed")
	}
}

type mockSession struct {
	mock.Mock
}

func newMockSession(t *testing.T) *mockSession {
	m := &mockSession{}
	m.Mock.Test(t)

	t.Cleanup(func() { m.AssertExpectations(t) })

	return m
}

func (m *mockSession) End() { m.Called() }

func (m *mockSession) Terminated() limiter.SessionTerminatedChan {
	return m.Called().Get(0).(limiter.SessionTerminatedChan)
}
