// Code generated by mockery v2.37.1. DO NOT EDIT.

package controllermock

import mock "github.com/stretchr/testify/mock"

// Lease is an autogenerated mock type for the Lease type
type Lease struct {
	mock.Mock
}

type Lease_Expecter struct {
	mock *mock.Mock
}

func (_m *Lease) EXPECT() *Lease_Expecter {
	return &Lease_Expecter{mock: &_m.Mock}
}

// Changed provides a mock function with given fields:
func (_m *Lease) Changed() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Lease_Changed_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Changed'
type Lease_Changed_Call struct {
	*mock.Call
}

// Changed is a helper method to define mock.On call
func (_e *Lease_Expecter) Changed() *Lease_Changed_Call {
	return &Lease_Changed_Call{Call: _e.mock.On("Changed")}
}

func (_c *Lease_Changed_Call) Run(run func()) *Lease_Changed_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Lease_Changed_Call) Return(_a0 <-chan struct{}) *Lease_Changed_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Lease_Changed_Call) RunAndReturn(run func() <-chan struct{}) *Lease_Changed_Call {
	_c.Call.Return(run)
	return _c
}

// Held provides a mock function with given fields:
func (_m *Lease) Held() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Lease_Held_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Held'
type Lease_Held_Call struct {
	*mock.Call
}

// Held is a helper method to define mock.On call
func (_e *Lease_Expecter) Held() *Lease_Held_Call {
	return &Lease_Held_Call{Call: _e.mock.On("Held")}
}

func (_c *Lease_Held_Call) Run(run func()) *Lease_Held_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Lease_Held_Call) Return(_a0 bool) *Lease_Held_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Lease_Held_Call) RunAndReturn(run func() bool) *Lease_Held_Call {
	_c.Call.Return(run)
	return _c
}

// NewLease creates a new instance of Lease. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewLease(t interface {
	mock.TestingT
	Cleanup(func())
}) *Lease {
	mock := &Lease{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
