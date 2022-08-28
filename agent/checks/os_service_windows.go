//go:build windows
// +build windows

package checks

import (
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var (
	win windowsSystem = windowsOS{}
)

type OSServiceClient struct {
	scHandle windows.Handle
}

func NewOSServiceClient() (*OSServiceClient, error) {
	scHandle, err := win.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		return nil, fmt.Errorf("error connecting to service manager: %w", err)
	}
	return &OSServiceClient{scHandle: scHandle}, nil
}

func (client *OSServiceClient) Check(serviceName string) (err error) {
	m := win.getWindowsSvcMgr(client.scHandle)
	defer m.Disconnect()

	svcNamePtr, err := syscall.UTF16PtrFromString(serviceName)
	if err != nil {
		return fmt.Errorf("service name must not contain NUL bytes: %w", err)
	}
	svcHandle, err := win.OpenService(win.getWindowsSvcMgrHandle(m), svcNamePtr, windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return fmt.Errorf("error accessing service: %w", err)
	}
	service := win.getWindowsSvc(serviceName, svcHandle)
	defer service.Close()

	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("error querying service status: %w", err)
	}

	switch status.State {
	case svc.Running:
		return nil
	case svc.Paused, svc.Stopped:
		err = fmt.Errorf("service status: %v - %w", svcStateString(status.State), ErrOSServiceStatusCritical)
	default:
		err = fmt.Errorf("service status: %v", svcStateString(status.State))
	}

	return err
}

type windowsOS struct{}

func (windowsOS) OpenSCManager(machineName *uint16, databaseName *uint16, access uint32) (handle windows.Handle, err error) {
	return windows.OpenSCManager(machineName, databaseName, access)
}
func (windowsOS) OpenService(mgr windows.Handle, serviceName *uint16, access uint32) (handle windows.Handle, err error) {
	return windows.OpenService(mgr, serviceName, access)
}

func (windowsOS) getWindowsSvcMgr(h windows.Handle) windowsSvcMgr { return &mgr.Mgr{Handle: h} }
func (windowsOS) getWindowsSvcMgrHandle(sm windowsSvcMgr) windows.Handle {
	return sm.(*mgr.Mgr).Handle
}

func (windowsOS) getWindowsSvc(name string, h windows.Handle) windowsSvc {
	return &mgr.Service{Name: name, Handle: h}
}

type windowsSystem interface {
	OpenSCManager(machineName *uint16, databaseName *uint16, access uint32) (handle windows.Handle, err error)
	OpenService(mgr windows.Handle, serviceName *uint16, access uint32) (handle windows.Handle, err error)

	getWindowsSvcMgr(h windows.Handle) windowsSvcMgr
	getWindowsSvcMgrHandle(sm windowsSvcMgr) windows.Handle
	getWindowsSvc(name string, h windows.Handle) windowsSvc
}

type windowsSvcMgr interface {
	Disconnect() error
}

type windowsSvc interface {
	Close() error
	Query() (svc.Status, error)
}

// svcStateString converts svc.State (uint32) to human readable string
//
// source: https://pkg.go.dev/golang.org/x/sys/windows/svc#pkg-constants
func svcStateString(state svc.State) string {
	switch state {
	case svc.State(windows.SERVICE_STOPPED):
		return "Stopped"
	case svc.State(windows.SERVICE_START_PENDING):
		return "StartPending"
	case svc.State(windows.SERVICE_STOP_PENDING):
		return "StopPending"
	case svc.State(windows.SERVICE_RUNNING):
		return "Running"
	case svc.State(windows.SERVICE_CONTINUE_PENDING):
		return "ContinuePending"
	case svc.State(windows.SERVICE_PAUSE_PENDING):
		return "PausePending"
	case svc.State(windows.SERVICE_PAUSED):
		return "Paused"
	default:
		//if not handled we return the underlying uint32
		return fmt.Sprintf("%d", state)
	}
}
