// Code generated by mockery v2.37.1. DO NOT EDIT.

package reportingmock

import (
	memdb "github.com/hashicorp/go-memdb"
	mock "github.com/stretchr/testify/mock"

	state "github.com/hashicorp/consul/agent/consul/state"

	structs "github.com/hashicorp/consul/agent/structs"
)

// StateDelegate is an autogenerated mock type for the StateDelegate type
type StateDelegate struct {
	mock.Mock
}

type StateDelegate_Expecter struct {
	mock *mock.Mock
}

func (_m *StateDelegate) EXPECT() *StateDelegate_Expecter {
	return &StateDelegate_Expecter{mock: &_m.Mock}
}

// NodeUsage provides a mock function with given fields:
func (_m *StateDelegate) NodeUsage() (uint64, state.NodeUsage, error) {
	ret := _m.Called()

	var r0 uint64
	var r1 state.NodeUsage
	var r2 error
	if rf, ok := ret.Get(0).(func() (uint64, state.NodeUsage, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func() state.NodeUsage); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(state.NodeUsage)
	}

	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// StateDelegate_NodeUsage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NodeUsage'
type StateDelegate_NodeUsage_Call struct {
	*mock.Call
}

// NodeUsage is a helper method to define mock.On call
func (_e *StateDelegate_Expecter) NodeUsage() *StateDelegate_NodeUsage_Call {
	return &StateDelegate_NodeUsage_Call{Call: _e.mock.On("NodeUsage")}
}

func (_c *StateDelegate_NodeUsage_Call) Run(run func()) *StateDelegate_NodeUsage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *StateDelegate_NodeUsage_Call) Return(_a0 uint64, _a1 state.NodeUsage, _a2 error) *StateDelegate_NodeUsage_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *StateDelegate_NodeUsage_Call) RunAndReturn(run func() (uint64, state.NodeUsage, error)) *StateDelegate_NodeUsage_Call {
	_c.Call.Return(run)
	return _c
}

// ServiceUsage provides a mock function with given fields: ws, tenantUsage
func (_m *StateDelegate) ServiceUsage(ws memdb.WatchSet, tenantUsage bool) (uint64, structs.ServiceUsage, error) {
	ret := _m.Called(ws, tenantUsage)

	var r0 uint64
	var r1 structs.ServiceUsage
	var r2 error
	if rf, ok := ret.Get(0).(func(memdb.WatchSet, bool) (uint64, structs.ServiceUsage, error)); ok {
		return rf(ws, tenantUsage)
	}
	if rf, ok := ret.Get(0).(func(memdb.WatchSet, bool) uint64); ok {
		r0 = rf(ws, tenantUsage)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(memdb.WatchSet, bool) structs.ServiceUsage); ok {
		r1 = rf(ws, tenantUsage)
	} else {
		r1 = ret.Get(1).(structs.ServiceUsage)
	}

	if rf, ok := ret.Get(2).(func(memdb.WatchSet, bool) error); ok {
		r2 = rf(ws, tenantUsage)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// StateDelegate_ServiceUsage_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ServiceUsage'
type StateDelegate_ServiceUsage_Call struct {
	*mock.Call
}

// ServiceUsage is a helper method to define mock.On call
//   - ws memdb.WatchSet
//   - tenantUsage bool
func (_e *StateDelegate_Expecter) ServiceUsage(ws interface{}, tenantUsage interface{}) *StateDelegate_ServiceUsage_Call {
	return &StateDelegate_ServiceUsage_Call{Call: _e.mock.On("ServiceUsage", ws, tenantUsage)}
}

func (_c *StateDelegate_ServiceUsage_Call) Run(run func(ws memdb.WatchSet, tenantUsage bool)) *StateDelegate_ServiceUsage_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(memdb.WatchSet), args[1].(bool))
	})
	return _c
}

func (_c *StateDelegate_ServiceUsage_Call) Return(_a0 uint64, _a1 structs.ServiceUsage, _a2 error) *StateDelegate_ServiceUsage_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *StateDelegate_ServiceUsage_Call) RunAndReturn(run func(memdb.WatchSet, bool) (uint64, structs.ServiceUsage, error)) *StateDelegate_ServiceUsage_Call {
	_c.Call.Return(run)
	return _c
}

// NewStateDelegate creates a new instance of StateDelegate. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewStateDelegate(t interface {
	mock.TestingT
	Cleanup(func())
}) *StateDelegate {
	mock := &StateDelegate{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
