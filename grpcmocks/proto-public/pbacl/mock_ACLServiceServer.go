// Code generated by mockery v2.37.1. DO NOT EDIT.

package mockpbacl

import (
	context "context"

	pbacl "github.com/hashicorp/consul/proto-public/pbacl"
	mock "github.com/stretchr/testify/mock"
)

// ACLServiceServer is an autogenerated mock type for the ACLServiceServer type
type ACLServiceServer struct {
	mock.Mock
}

type ACLServiceServer_Expecter struct {
	mock *mock.Mock
}

func (_m *ACLServiceServer) EXPECT() *ACLServiceServer_Expecter {
	return &ACLServiceServer_Expecter{mock: &_m.Mock}
}

// Login provides a mock function with given fields: _a0, _a1
func (_m *ACLServiceServer) Login(_a0 context.Context, _a1 *pbacl.LoginRequest) (*pbacl.LoginResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *pbacl.LoginResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pbacl.LoginRequest) (*pbacl.LoginResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pbacl.LoginRequest) *pbacl.LoginResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbacl.LoginResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pbacl.LoginRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ACLServiceServer_Login_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Login'
type ACLServiceServer_Login_Call struct {
	*mock.Call
}

// Login is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *pbacl.LoginRequest
func (_e *ACLServiceServer_Expecter) Login(_a0 interface{}, _a1 interface{}) *ACLServiceServer_Login_Call {
	return &ACLServiceServer_Login_Call{Call: _e.mock.On("Login", _a0, _a1)}
}

func (_c *ACLServiceServer_Login_Call) Run(run func(_a0 context.Context, _a1 *pbacl.LoginRequest)) *ACLServiceServer_Login_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*pbacl.LoginRequest))
	})
	return _c
}

func (_c *ACLServiceServer_Login_Call) Return(_a0 *pbacl.LoginResponse, _a1 error) *ACLServiceServer_Login_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ACLServiceServer_Login_Call) RunAndReturn(run func(context.Context, *pbacl.LoginRequest) (*pbacl.LoginResponse, error)) *ACLServiceServer_Login_Call {
	_c.Call.Return(run)
	return _c
}

// Logout provides a mock function with given fields: _a0, _a1
func (_m *ACLServiceServer) Logout(_a0 context.Context, _a1 *pbacl.LogoutRequest) (*pbacl.LogoutResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *pbacl.LogoutResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pbacl.LogoutRequest) (*pbacl.LogoutResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pbacl.LogoutRequest) *pbacl.LogoutResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbacl.LogoutResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pbacl.LogoutRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ACLServiceServer_Logout_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Logout'
type ACLServiceServer_Logout_Call struct {
	*mock.Call
}

// Logout is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *pbacl.LogoutRequest
func (_e *ACLServiceServer_Expecter) Logout(_a0 interface{}, _a1 interface{}) *ACLServiceServer_Logout_Call {
	return &ACLServiceServer_Logout_Call{Call: _e.mock.On("Logout", _a0, _a1)}
}

func (_c *ACLServiceServer_Logout_Call) Run(run func(_a0 context.Context, _a1 *pbacl.LogoutRequest)) *ACLServiceServer_Logout_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*pbacl.LogoutRequest))
	})
	return _c
}

func (_c *ACLServiceServer_Logout_Call) Return(_a0 *pbacl.LogoutResponse, _a1 error) *ACLServiceServer_Logout_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ACLServiceServer_Logout_Call) RunAndReturn(run func(context.Context, *pbacl.LogoutRequest) (*pbacl.LogoutResponse, error)) *ACLServiceServer_Logout_Call {
	_c.Call.Return(run)
	return _c
}

// NewACLServiceServer creates a new instance of ACLServiceServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewACLServiceServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *ACLServiceServer {
	mock := &ACLServiceServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
