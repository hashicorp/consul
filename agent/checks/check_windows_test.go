// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build windows
// +build windows

package checks

import (
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func TestCheck_OSService(t *testing.T) {
	type args struct {
		returnsOpenSCManagerError   error
		returnsOpenServiceError     error
		returnsServiceQueryError    error
		returnsServiceCloseError    error
		returnsSCMgrDisconnectError error
		returnsServiceState         svc.State
	}
	tests := []struct {
		desc  string
		args  args
		state string
	}{
		//healthy
		{"should pass for healthy service", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Running,
		}, api.HealthPassing},
		{"should pass for healthy service even when there's an error closing the service handle", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    errors.New("error while closing the service handle"),
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Running,
		}, api.HealthPassing},
		{"should pass for healthy service even when there's an error disconnecting from SCManager", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: errors.New("error while disconnecting from service manager"),
			returnsServiceState:         svc.Running,
		}, api.HealthPassing},

		// // warning
		{"should be in warning state for any state that's not Running, Paused or Stopped", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.StartPending,
		}, api.HealthWarning},
		{"should be in warning state when we cannot connect to the service manager", args{
			returnsOpenSCManagerError:   errors.New("cannot connect to service manager"),
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Running,
		}, api.HealthWarning},
		{"should be in warning state when we cannot open the service", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     errors.New("service testService does not exist"),
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Running,
		}, api.HealthWarning},
		{"should be in warning state when we cannot query the service state", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    errors.New("cannot query testService state"),
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Running,
		}, api.HealthWarning},
		{"should be in warning state for for any state that's not Running, Paused or Stopped when there's an error closing the service handle", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    errors.New("error while closing the service handle"),
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.StartPending,
		}, api.HealthWarning},
		{"should be in warning state for for any state that's not Running, Paused or Stopped when there's an error disconnecting from SCManager", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: errors.New("error while disconnecting from service manager"),
			returnsServiceState:         svc.StartPending,
		}, api.HealthWarning},

		// critical
		{"should fail for paused service", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Paused,
		}, api.HealthCritical},
		{"should fail for stopped service", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Stopped,
		}, api.HealthCritical},
		{"should fail for stopped service even when there's an error closing the service handle", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    errors.New("error while closing the service handle"),
			returnsSCMgrDisconnectError: nil,
			returnsServiceState:         svc.Stopped,
		}, api.HealthCritical},
		{"should fail for stopped service even when there's an error disconnecting from SCManager", args{
			returnsOpenSCManagerError:   nil,
			returnsOpenServiceError:     nil,
			returnsServiceQueryError:    nil,
			returnsServiceCloseError:    nil,
			returnsSCMgrDisconnectError: errors.New("error while disconnecting from service manager"),
			returnsServiceState:         svc.Stopped,
		}, api.HealthCritical},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			old := win
			defer func() { win = old }()
			win = fakeWindowsOS{
				returnsOpenSCManagerError:   tt.args.returnsOpenSCManagerError,
				returnsOpenServiceError:     tt.args.returnsOpenServiceError,
				returnsServiceQueryError:    tt.args.returnsServiceQueryError,
				returnsServiceCloseError:    tt.args.returnsServiceCloseError,
				returnsSCMgrDisconnectError: tt.args.returnsSCMgrDisconnectError,
				returnsServiceState:         tt.args.returnsServiceState,
			}
			c, err := NewOSServiceClient()
			if (tt.args.returnsOpenSCManagerError != nil && err == nil) || (tt.args.returnsOpenSCManagerError == nil && err != nil) {
				t.Errorf("FAIL: %s. Expected error on OpenSCManager %v , but err == %v", tt.desc, tt.args.returnsOpenSCManagerError, err)
			}
			if err != nil {
				return
			}

			notif, upd := mock.NewNotifyChan()
			logger := testutil.Logger(t)
			statusHandler := NewStatusHandler(notif, logger, 0, 0, 0)
			id := structs.NewCheckID("chk", nil)

			check := &CheckOSService{
				CheckID:       id,
				OSService:     "testService",
				Interval:      25 * time.Millisecond,
				Client:        c,
				Logger:        logger,
				StatusHandler: statusHandler,
			}
			check.Start()
			defer check.Stop()

			<-upd // wait for update

			if got, want := notif.State(id), tt.state; got != want {
				t.Fatalf("got status %q want %q", got, want)
			}
		})
	}
}

const (
	validSCManagerHandle   = windows.Handle(1)
	validOpenServiceHandle = windows.Handle(2)
)

type fakeWindowsOS struct {
	returnsOpenSCManagerError   error
	returnsOpenServiceError     error
	returnsServiceQueryError    error
	returnsServiceCloseError    error
	returnsSCMgrDisconnectError error
	returnsServiceState         svc.State
}

func (f fakeWindowsOS) OpenSCManager(machineName *uint16, databaseName *uint16, access uint32) (handle windows.Handle, err error) {
	if f.returnsOpenSCManagerError != nil {
		return windows.InvalidHandle, f.returnsOpenSCManagerError
	}
	return validSCManagerHandle, nil
}
func (f fakeWindowsOS) OpenService(mgr windows.Handle, serviceName *uint16, access uint32) (handle windows.Handle, err error) {
	if f.returnsOpenServiceError != nil {
		return windows.InvalidHandle, f.returnsOpenServiceError
	}
	return validOpenServiceHandle, nil
}

func (f fakeWindowsOS) getWindowsSvcMgr(h windows.Handle) windowsSvcMgr {
	return &fakeWindowsSvcMgr{
		Handle:                 h,
		returnsDisconnectError: f.returnsSCMgrDisconnectError,
	}
}
func (fakeWindowsOS) getWindowsSvcMgrHandle(sm windowsSvcMgr) windows.Handle {
	return sm.(*fakeWindowsSvcMgr).Handle
}

func (f fakeWindowsOS) getWindowsSvc(name string, h windows.Handle) windowsSvc {
	return &fakeWindowsSvc{
		Name:                     name,
		Handle:                   h,
		returnsCloseError:        f.returnsServiceCloseError,
		returnsServiceQueryError: f.returnsServiceQueryError,
		returnsServiceState:      f.returnsServiceState,
	}
}

type fakeWindowsSvcMgr struct {
	Handle windows.Handle

	returnsDisconnectError error
}

func (f fakeWindowsSvcMgr) Disconnect() error { return f.returnsDisconnectError }

type fakeWindowsSvc struct {
	Handle windows.Handle
	Name   string

	returnsServiceQueryError error
	returnsCloseError        error
	returnsServiceState      svc.State
}

func (f fakeWindowsSvc) Close() error { return f.returnsCloseError }
func (f fakeWindowsSvc) Query() (svc.Status, error) {
	if f.returnsServiceQueryError != nil {
		return svc.Status{}, f.returnsServiceQueryError
	}
	return svc.Status{State: f.returnsServiceState}, nil
}

func boolPointer(b bool) *bool {
	return &b
}

func boolVal(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}
