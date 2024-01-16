// Code generated by mockery v2.37.1. DO NOT EDIT.

package mockpbconnectca

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	pbconnectca "github.com/hashicorp/consul/proto-public/pbconnectca"
)

// ConnectCAServiceClient is an autogenerated mock type for the ConnectCAServiceClient type
type ConnectCAServiceClient struct {
	mock.Mock
}

type ConnectCAServiceClient_Expecter struct {
	mock *mock.Mock
}

func (_m *ConnectCAServiceClient) EXPECT() *ConnectCAServiceClient_Expecter {
	return &ConnectCAServiceClient_Expecter{mock: &_m.Mock}
}

// Sign provides a mock function with given fields: ctx, in, opts
func (_m *ConnectCAServiceClient) Sign(ctx context.Context, in *pbconnectca.SignRequest, opts ...grpc.CallOption) (*pbconnectca.SignResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pbconnectca.SignResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pbconnectca.SignRequest, ...grpc.CallOption) (*pbconnectca.SignResponse, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pbconnectca.SignRequest, ...grpc.CallOption) *pbconnectca.SignResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbconnectca.SignResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pbconnectca.SignRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ConnectCAServiceClient_Sign_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Sign'
type ConnectCAServiceClient_Sign_Call struct {
	*mock.Call
}

// Sign is a helper method to define mock.On call
//   - ctx context.Context
//   - in *pbconnectca.SignRequest
//   - opts ...grpc.CallOption
func (_e *ConnectCAServiceClient_Expecter) Sign(ctx interface{}, in interface{}, opts ...interface{}) *ConnectCAServiceClient_Sign_Call {
	return &ConnectCAServiceClient_Sign_Call{Call: _e.mock.On("Sign",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *ConnectCAServiceClient_Sign_Call) Run(run func(ctx context.Context, in *pbconnectca.SignRequest, opts ...grpc.CallOption)) *ConnectCAServiceClient_Sign_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*pbconnectca.SignRequest), variadicArgs...)
	})
	return _c
}

func (_c *ConnectCAServiceClient_Sign_Call) Return(_a0 *pbconnectca.SignResponse, _a1 error) *ConnectCAServiceClient_Sign_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ConnectCAServiceClient_Sign_Call) RunAndReturn(run func(context.Context, *pbconnectca.SignRequest, ...grpc.CallOption) (*pbconnectca.SignResponse, error)) *ConnectCAServiceClient_Sign_Call {
	_c.Call.Return(run)
	return _c
}

// WatchRoots provides a mock function with given fields: ctx, in, opts
func (_m *ConnectCAServiceClient) WatchRoots(ctx context.Context, in *pbconnectca.WatchRootsRequest, opts ...grpc.CallOption) (pbconnectca.ConnectCAService_WatchRootsClient, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 pbconnectca.ConnectCAService_WatchRootsClient
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pbconnectca.WatchRootsRequest, ...grpc.CallOption) (pbconnectca.ConnectCAService_WatchRootsClient, error)); ok {
		return rf(ctx, in, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pbconnectca.WatchRootsRequest, ...grpc.CallOption) pbconnectca.ConnectCAService_WatchRootsClient); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pbconnectca.ConnectCAService_WatchRootsClient)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pbconnectca.WatchRootsRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ConnectCAServiceClient_WatchRoots_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'WatchRoots'
type ConnectCAServiceClient_WatchRoots_Call struct {
	*mock.Call
}

// WatchRoots is a helper method to define mock.On call
//   - ctx context.Context
//   - in *pbconnectca.WatchRootsRequest
//   - opts ...grpc.CallOption
func (_e *ConnectCAServiceClient_Expecter) WatchRoots(ctx interface{}, in interface{}, opts ...interface{}) *ConnectCAServiceClient_WatchRoots_Call {
	return &ConnectCAServiceClient_WatchRoots_Call{Call: _e.mock.On("WatchRoots",
		append([]interface{}{ctx, in}, opts...)...)}
}

func (_c *ConnectCAServiceClient_WatchRoots_Call) Run(run func(ctx context.Context, in *pbconnectca.WatchRootsRequest, opts ...grpc.CallOption)) *ConnectCAServiceClient_WatchRoots_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]grpc.CallOption, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(grpc.CallOption)
			}
		}
		run(args[0].(context.Context), args[1].(*pbconnectca.WatchRootsRequest), variadicArgs...)
	})
	return _c
}

func (_c *ConnectCAServiceClient_WatchRoots_Call) Return(_a0 pbconnectca.ConnectCAService_WatchRootsClient, _a1 error) *ConnectCAServiceClient_WatchRoots_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ConnectCAServiceClient_WatchRoots_Call) RunAndReturn(run func(context.Context, *pbconnectca.WatchRootsRequest, ...grpc.CallOption) (pbconnectca.ConnectCAService_WatchRootsClient, error)) *ConnectCAServiceClient_WatchRoots_Call {
	_c.Call.Return(run)
	return _c
}

// NewConnectCAServiceClient creates a new instance of ConnectCAServiceClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewConnectCAServiceClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *ConnectCAServiceClient {
	mock := &ConnectCAServiceClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
