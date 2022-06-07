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

type OSServiceClient struct {
	scHandle windows.Handle
}

func NewOSServiceClient() (*OSServiceClient, error) {
	var s *uint16
	scHandle, err := windows.OpenSCManager(s, nil, windows.SC_MANAGER_CONNECT)

	if err != nil {
		return nil, fmt.Errorf("error connecting to service manager: %w", err)
	}

	return &OSServiceClient{
		scHandle: scHandle,
	}, nil
}

func (client *OSServiceClient) Check(serviceName string) error {
	m := &mgr.Mgr{Handle: client.scHandle}
	defer m.Disconnect()
	svcHandle, err := windows.OpenService(m.Handle, syscall.StringToUTF16Ptr(serviceName), windows.SC_MANAGER_ENUMERATE_SERVICE)
	if err != nil {
		return fmt.Errorf("error accessing service: %w", err)
	}
	service := &mgr.Service{Name: serviceName, Handle: svcHandle}
	defer service.Close()
	status, err := service.Query()
	if err != nil {
		return fmt.Errorf("error querying service status: %w", err)
	}

	switch status.State {
	case svc.Running:
		return nil
	case svc.Stopped:
		return ErrOSServiceStatusCritical
	default:
		return fmt.Errorf("service status: %v", status.State)
	}
}
