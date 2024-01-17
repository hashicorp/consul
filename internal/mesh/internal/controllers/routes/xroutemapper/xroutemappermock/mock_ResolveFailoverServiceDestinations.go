// Code generated by mockery v2.37.1. DO NOT EDIT.

package xroutemappermock

import (
	context "context"

	controller "github.com/hashicorp/consul/internal/controller"
	mock "github.com/stretchr/testify/mock"

	pbresource "github.com/hashicorp/consul/proto-public/pbresource"
)

// ResolveFailoverServiceDestinations is an autogenerated mock type for the ResolveFailoverServiceDestinations type
type ResolveFailoverServiceDestinations struct {
	mock.Mock
}

type ResolveFailoverServiceDestinations_Expecter struct {
	mock *mock.Mock
}

func (_m *ResolveFailoverServiceDestinations) EXPECT() *ResolveFailoverServiceDestinations_Expecter {
	return &ResolveFailoverServiceDestinations_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: _a0, _a1, _a2
func (_m *ResolveFailoverServiceDestinations) Execute(_a0 context.Context, _a1 controller.Runtime, _a2 *pbresource.ID) ([]*pbresource.ID, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 []*pbresource.ID
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, controller.Runtime, *pbresource.ID) ([]*pbresource.ID, error)); ok {
		return rf(_a0, _a1, _a2)
	}
	if rf, ok := ret.Get(0).(func(context.Context, controller.Runtime, *pbresource.ID) []*pbresource.ID); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*pbresource.ID)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, controller.Runtime, *pbresource.ID) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ResolveFailoverServiceDestinations_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type ResolveFailoverServiceDestinations_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 controller.Runtime
//   - _a2 *pbresource.ID
func (_e *ResolveFailoverServiceDestinations_Expecter) Execute(_a0 interface{}, _a1 interface{}, _a2 interface{}) *ResolveFailoverServiceDestinations_Execute_Call {
	return &ResolveFailoverServiceDestinations_Execute_Call{Call: _e.mock.On("Execute", _a0, _a1, _a2)}
}

func (_c *ResolveFailoverServiceDestinations_Execute_Call) Run(run func(_a0 context.Context, _a1 controller.Runtime, _a2 *pbresource.ID)) *ResolveFailoverServiceDestinations_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(controller.Runtime), args[2].(*pbresource.ID))
	})
	return _c
}

func (_c *ResolveFailoverServiceDestinations_Execute_Call) Return(_a0 []*pbresource.ID, _a1 error) *ResolveFailoverServiceDestinations_Execute_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *ResolveFailoverServiceDestinations_Execute_Call) RunAndReturn(run func(context.Context, controller.Runtime, *pbresource.ID) ([]*pbresource.ID, error)) *ResolveFailoverServiceDestinations_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewResolveFailoverServiceDestinations creates a new instance of ResolveFailoverServiceDestinations. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewResolveFailoverServiceDestinations(t interface {
	mock.TestingT
	Cleanup(func())
}) *ResolveFailoverServiceDestinations {
	mock := &ResolveFailoverServiceDestinations{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
