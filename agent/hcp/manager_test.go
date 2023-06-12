package hcp

import (
	"io"
	"testing"
	"time"

	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestManager_Run(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	updateCh := make(chan struct{}, 1)
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Once()
	mgr := NewManager(ManagerConfig{
		Client:   client,
		Logger:   hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn: statusF,
	})
	mgr.testUpdateSent = updateCh
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go mgr.Run(ctx)
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}

	// Make sure after manager has stopped no more statuses are pushed.
	cancel()
	client.AssertExpectations(t)
}

func TestManager_SendUpdate(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	updateCh := make(chan struct{}, 1)

	// Expect two calls, once during run startup and again when SendUpdate is called
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Twice()
	mgr := NewManager(ManagerConfig{
		Client:   client,
		Logger:   hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn: statusF,
	})
	mgr.testUpdateSent = updateCh
	go mgr.Run(context.Background())
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	mgr.SendUpdate()
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	client.AssertExpectations(t)
}

func TestManager_SendUpdate_Periodic(t *testing.T) {
	client := hcpclient.NewMockClient(t)
	statusF := func(ctx context.Context) (hcpclient.ServerStatus, error) {
		return hcpclient.ServerStatus{ID: t.Name()}, nil
	}
	updateCh := make(chan struct{}, 1)

	// Expect two calls, once during run startup and again when SendUpdate is called
	client.EXPECT().PushServerStatus(mock.Anything, &hcpclient.ServerStatus{ID: t.Name()}).Return(nil).Twice()
	mgr := NewManager(ManagerConfig{
		Client:      client,
		Logger:      hclog.New(&hclog.LoggerOptions{Output: io.Discard}),
		StatusFn:    statusF,
		MaxInterval: time.Second,
		MinInterval: 100 * time.Millisecond,
	})
	mgr.testUpdateSent = updateCh
	go mgr.Run(context.Background())
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	select {
	case <-updateCh:
	case <-time.After(time.Second):
		require.Fail(t, "manager did not send update in expected time")
	}
	client.AssertExpectations(t)
}
