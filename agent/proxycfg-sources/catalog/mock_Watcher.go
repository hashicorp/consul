// Code generated by mockery v2.32.4. DO NOT EDIT.

package catalog

import (
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/internal/mesh/proxy-snapshot"
	mock "github.com/stretchr/testify/mock"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
)

// MockWatcher is an autogenerated mock type for the Watcher type
type MockWatcher struct {
	mock.Mock
}

// Watch provides a mock function with given fields: proxyID, nodeName, token
func (_m *MockWatcher) Watch(proxyID *pbresource.ID, nodeName string, token string) (<-chan proxysnapshot.ProxySnapshot, limiter.SessionTerminatedChan, proxysnapshot.CancelFunc, error) {
	ret := _m.Called(proxyID, nodeName, token)

	var r0 <-chan proxysnapshot.ProxySnapshot
	var r1 limiter.SessionTerminatedChan
	var r2 proxysnapshot.CancelFunc
	var r3 error
	if rf, ok := ret.Get(0).(func(*pbresource.ID, string, string) (<-chan proxysnapshot.ProxySnapshot, limiter.SessionTerminatedChan, proxysnapshot.CancelFunc, error)); ok {
		return rf(proxyID, nodeName, token)
	}
	if rf, ok := ret.Get(0).(func(*pbresource.ID, string, string) <-chan proxysnapshot.ProxySnapshot); ok {
		r0 = rf(proxyID, nodeName, token)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan proxysnapshot.ProxySnapshot)
		}
	}

	if rf, ok := ret.Get(1).(func(*pbresource.ID, string, string) limiter.SessionTerminatedChan); ok {
		r1 = rf(proxyID, nodeName, token)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(limiter.SessionTerminatedChan)
		}
	}

	if rf, ok := ret.Get(2).(func(*pbresource.ID, string, string) proxysnapshot.CancelFunc); ok {
		r2 = rf(proxyID, nodeName, token)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(proxysnapshot.CancelFunc)
		}
	}

	if rf, ok := ret.Get(3).(func(*pbresource.ID, string, string) error); ok {
		r3 = rf(proxyID, nodeName, token)
	} else {
		r3 = ret.Error(3)
	}

	return r0, r1, r2, r3
}

// NewMockWatcher creates a new instance of MockWatcher. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockWatcher(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockWatcher {
	mock := &MockWatcher{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
