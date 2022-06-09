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
	var s *uint16
	scHandle, err := win.OpenSCManager(s, nil, windows.SC_MANAGER_CONNECT)

	if err != nil {
		return nil, fmt.Errorf("error connecting to service manager: %w", err)
	}

	return &OSServiceClient{
		scHandle: scHandle,
	}, nil
}

func (client *OSServiceClient) Check(serviceName string) (err error) {
	var isHealthy bool

	m := win.getWindowsSvcMgr(client.scHandle)
	defer func() {
		errDisconnect := m.Disconnect()
		if isHealthy || errDisconnect == nil || err != nil {
			return
		}
		//unreachable at the moment but we might want to log this error. leaving here for code-review
		err = errDisconnect
	}()

	svcHandle, err := win.OpenService(win.getWindowsSvcMgrHandle(m), syscall.StringToUTF16Ptr(serviceName), windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return fmt.Errorf("error accessing service: %w", err)
	}
	service := win.getWindowsSvc(serviceName, svcHandle)
	defer func() {
		errClose := service.Close()
		if isHealthy || errClose == nil || err != nil {
			return
		}
		//unreachable at the moment but we might want to log this error. leaving here for code-review
		err = errClose
	}()
	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("error querying service status: %w", err)
	}

	switch status.State {
	case svc.Running:
		err = nil
		isHealthy = true
	case svc.Paused, svc.Stopped:
		err = ErrOSServiceStatusCritical
	default:
		err = fmt.Errorf("service status: %v", status.State)
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
