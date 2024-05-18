// Code generated by mockery v2.41.0. DO NOT EDIT.

package mockpbserverdiscovery

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	metadata "google.golang.org/grpc/metadata"

	pbserverdiscovery "github.com/hashicorp/consul/proto-public/pbserverdiscovery"
)

// ServerDiscoveryService_WatchServersClient is an autogenerated mock type for the ServerDiscoveryService_WatchServersClient type
type ServerDiscoveryService_WatchServersClient struct {
	mock.Mock
}

type ServerDiscoveryService_WatchServersClient_Expecter struct {
	mock *mock.Mock
}

func (_m *ServerDiscoveryService_WatchServersClient) EXPECT() *ServerDiscoveryService_WatchServersClient_Expecter {
	return &ServerDiscoveryService_WatchServersClient_Expecter{mock: &_m.Mock}
}

// CloseSend provides a mock function with given fields:
func (_m *ServerDiscoveryService_WatchServersClient) CloseSend() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CloseSend")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ServerDiscoveryService_WatchServersClient_CloseSend_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'CloseSend'
type ServerDiscoveryService_WatchServersClient_CloseSend_Call struct {
	*mock.Call
}

// CloseSend is a helper method to define mock.On call
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) CloseSend() *ServerDiscoveryService_WatchServersClient_CloseSend_Call {
	return &ServerDiscoveryService_WatchServersClient_CloseSend_Call{Call: _e.mock.On("CloseSend")}
}

func (_c *ServerDiscoveryService_WatchServersClient_CloseSend_Call) Run(run func()) *ServerDiscoveryService_WatchServersClient_CloseSend_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_CloseSend_Call) Return(_a0 error) *ServerDiscoveryService_WatchServersClient_CloseSend_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_CloseSend_Call) RunAndReturn(run func() error) *ServerDiscoveryService_WatchServersClient_CloseSend_Call {
	_c.Call.Return(run)
	return _c
}

// Context provides a mock function with given fields:
func (_m *ServerDiscoveryService_WatchServersClient) Context() context.Context {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Context")
	}

	var r0 context.Context
	if rf, ok := ret.Get(0).(func() context.Context); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// ServerDiscoveryService_WatchServersClient_Context_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Context'
type ServerDiscoveryService_WatchServersClient_Context_Call struct {
	*mock.Call
}

// Context is a helper method to define mock.On call
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) Context() *ServerDiscoveryService_WatchServersClient_Context_Call {
	return &ServerDiscoveryService_WatchServersClient_Context_Call{Call: _e.mock.On("Context")}
}

func (_c *ServerDiscoveryService_WatchServersClient_Context_Call) Run(run func()) *ServerDiscoveryService_WatchServersClient_Context_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Context_Call) Return(_a0 context.Context) *ServerDiscoveryService_WatchServersClient_Context_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Context_Call) RunAndReturn(run func() context.Context) *ServerDiscoveryService_WatchServersClient_Context_Call {
	_c.Call.Return(run)
	return _c
}

// Header provides a mock function with given fields:
func (_m *ServerDiscoveryService_WatchServersClient) Header() (metadata.MD, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Header")
	}

	var r0 metadata.MD
	var r1 error
	if rf, ok := ret.Get(0).(func() (metadata.MD, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() metadata.MD); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metadata.MD)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ServerDiscoveryService_WatchServersClient_Header_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Header'
type ServerDiscoveryService_WatchServersClient_Header_Call struct {
	*mock.Call
}

// Header is a helper method to define mock.On call
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) Header() *ServerDiscoveryService_WatchServersClient_Header_Call {
	return &ServerDiscoveryService_WatchServersClient_Header_Call{Call: _e.mock.On("Header")}
}

func (_c *ServerDiscoveryService_WatchServersClient_Header_Call) Run(run func()) *ServerDiscoveryService_WatchServersClient_Header_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Header_Call) Return(_a0 metadata.MD, _a1 error) *ServerDiscoveryService_WatchServersClient_Header_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Header_Call) RunAndReturn(run func() (metadata.MD, error)) *ServerDiscoveryService_WatchServersClient_Header_Call {
	_c.Call.Return(run)
	return _c
}

// Recv provides a mock function with given fields:
func (_m *ServerDiscoveryService_WatchServersClient) Recv() (*pbserverdiscovery.WatchServersResponse, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Recv")
	}

	var r0 *pbserverdiscovery.WatchServersResponse
	var r1 error
	if rf, ok := ret.Get(0).(func() (*pbserverdiscovery.WatchServersResponse, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *pbserverdiscovery.WatchServersResponse); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbserverdiscovery.WatchServersResponse)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ServerDiscoveryService_WatchServersClient_Recv_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Recv'
type ServerDiscoveryService_WatchServersClient_Recv_Call struct {
	*mock.Call
}

// Recv is a helper method to define mock.On call
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) Recv() *ServerDiscoveryService_WatchServersClient_Recv_Call {
	return &ServerDiscoveryService_WatchServersClient_Recv_Call{Call: _e.mock.On("Recv")}
}

func (_c *ServerDiscoveryService_WatchServersClient_Recv_Call) Run(run func()) *ServerDiscoveryService_WatchServersClient_Recv_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Recv_Call) Return(_a0 *pbserverdiscovery.WatchServersResponse, _a1 error) *ServerDiscoveryService_WatchServersClient_Recv_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Recv_Call) RunAndReturn(run func() (*pbserverdiscovery.WatchServersResponse, error)) *ServerDiscoveryService_WatchServersClient_Recv_Call {
	_c.Call.Return(run)
	return _c
}

// RecvMsg provides a mock function with given fields: m
func (_m *ServerDiscoveryService_WatchServersClient) RecvMsg(m interface{}) error {
	ret := _m.Called(m)

	if len(ret) == 0 {
		panic("no return value specified for RecvMsg")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ServerDiscoveryService_WatchServersClient_RecvMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'RecvMsg'
type ServerDiscoveryService_WatchServersClient_RecvMsg_Call struct {
	*mock.Call
}

// RecvMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) RecvMsg(m interface{}) *ServerDiscoveryService_WatchServersClient_RecvMsg_Call {
	return &ServerDiscoveryService_WatchServersClient_RecvMsg_Call{Call: _e.mock.On("RecvMsg", m)}
}

func (_c *ServerDiscoveryService_WatchServersClient_RecvMsg_Call) Run(run func(m interface{})) *ServerDiscoveryService_WatchServersClient_RecvMsg_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_RecvMsg_Call) Return(_a0 error) *ServerDiscoveryService_WatchServersClient_RecvMsg_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_RecvMsg_Call) RunAndReturn(run func(interface{}) error) *ServerDiscoveryService_WatchServersClient_RecvMsg_Call {
	_c.Call.Return(run)
	return _c
}

// SendMsg provides a mock function with given fields: m
func (_m *ServerDiscoveryService_WatchServersClient) SendMsg(m interface{}) error {
	ret := _m.Called(m)

	if len(ret) == 0 {
		panic("no return value specified for SendMsg")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ServerDiscoveryService_WatchServersClient_SendMsg_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SendMsg'
type ServerDiscoveryService_WatchServersClient_SendMsg_Call struct {
	*mock.Call
}

// SendMsg is a helper method to define mock.On call
//   - m interface{}
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) SendMsg(m interface{}) *ServerDiscoveryService_WatchServersClient_SendMsg_Call {
	return &ServerDiscoveryService_WatchServersClient_SendMsg_Call{Call: _e.mock.On("SendMsg", m)}
}

func (_c *ServerDiscoveryService_WatchServersClient_SendMsg_Call) Run(run func(m interface{})) *ServerDiscoveryService_WatchServersClient_SendMsg_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(interface{}))
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_SendMsg_Call) Return(_a0 error) *ServerDiscoveryService_WatchServersClient_SendMsg_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_SendMsg_Call) RunAndReturn(run func(interface{}) error) *ServerDiscoveryService_WatchServersClient_SendMsg_Call {
	_c.Call.Return(run)
	return _c
}

// Trailer provides a mock function with given fields:
func (_m *ServerDiscoveryService_WatchServersClient) Trailer() metadata.MD {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Trailer")
	}

	var r0 metadata.MD
	if rf, ok := ret.Get(0).(func() metadata.MD); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metadata.MD)
		}
	}

	return r0
}

// ServerDiscoveryService_WatchServersClient_Trailer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Trailer'
type ServerDiscoveryService_WatchServersClient_Trailer_Call struct {
	*mock.Call
}

// Trailer is a helper method to define mock.On call
func (_e *ServerDiscoveryService_WatchServersClient_Expecter) Trailer() *ServerDiscoveryService_WatchServersClient_Trailer_Call {
	return &ServerDiscoveryService_WatchServersClient_Trailer_Call{Call: _e.mock.On("Trailer")}
}

func (_c *ServerDiscoveryService_WatchServersClient_Trailer_Call) Run(run func()) *ServerDiscoveryService_WatchServersClient_Trailer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Trailer_Call) Return(_a0 metadata.MD) *ServerDiscoveryService_WatchServersClient_Trailer_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *ServerDiscoveryService_WatchServersClient_Trailer_Call) RunAndReturn(run func() metadata.MD) *ServerDiscoveryService_WatchServersClient_Trailer_Call {
	_c.Call.Return(run)
	return _c
}

// NewServerDiscoveryService_WatchServersClient creates a new instance of ServerDiscoveryService_WatchServersClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewServerDiscoveryService_WatchServersClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *ServerDiscoveryService_WatchServersClient {
	mock := &ServerDiscoveryService_WatchServersClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
