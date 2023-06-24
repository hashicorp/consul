// Code generated by mockery v2.20.0. DO NOT EDIT.

package scada

import (
	net "net"

	mock "github.com/stretchr/testify/mock"

	provider "github.com/hashicorp/hcp-scada-provider"

	time "time"
)

// MockProvider is an autogenerated mock type for the Provider type
type MockProvider struct {
	mock.Mock
}

type MockProvider_Expecter struct {
	mock *mock.Mock
}

func (_m *MockProvider) EXPECT() *MockProvider_Expecter {
	return &MockProvider_Expecter{mock: &_m.Mock}
}

// AddMeta provides a mock function with given fields: _a0
func (_m *MockProvider) AddMeta(_a0 ...provider.Meta) {
	_va := make([]interface{}, len(_a0))
	for _i := range _a0 {
		_va[_i] = _a0[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockProvider_AddMeta_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AddMeta'
type MockProvider_AddMeta_Call struct {
	*mock.Call
}

// AddMeta is a helper method to define mock.On call
//   - _a0 ...provider.Meta
func (_e *MockProvider_Expecter) AddMeta(_a0 ...interface{}) *MockProvider_AddMeta_Call {
	return &MockProvider_AddMeta_Call{Call: _e.mock.On("AddMeta",
		append([]interface{}{}, _a0...)...)}
}

func (_c *MockProvider_AddMeta_Call) Run(run func(_a0 ...provider.Meta)) *MockProvider_AddMeta_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]provider.Meta, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(provider.Meta)
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *MockProvider_AddMeta_Call) Return() *MockProvider_AddMeta_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockProvider_AddMeta_Call) RunAndReturn(run func(...provider.Meta)) *MockProvider_AddMeta_Call {
	_c.Call.Return(run)
	return _c
}

// DeleteMeta provides a mock function with given fields: _a0
func (_m *MockProvider) DeleteMeta(_a0 ...string) {
	_va := make([]interface{}, len(_a0))
	for _i := range _a0 {
		_va[_i] = _a0[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockProvider_DeleteMeta_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DeleteMeta'
type MockProvider_DeleteMeta_Call struct {
	*mock.Call
}

// DeleteMeta is a helper method to define mock.On call
//   - _a0 ...string
func (_e *MockProvider_Expecter) DeleteMeta(_a0 ...interface{}) *MockProvider_DeleteMeta_Call {
	return &MockProvider_DeleteMeta_Call{Call: _e.mock.On("DeleteMeta",
		append([]interface{}{}, _a0...)...)}
}

func (_c *MockProvider_DeleteMeta_Call) Run(run func(_a0 ...string)) *MockProvider_DeleteMeta_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]string, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(string)
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *MockProvider_DeleteMeta_Call) Return() *MockProvider_DeleteMeta_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockProvider_DeleteMeta_Call) RunAndReturn(run func(...string)) *MockProvider_DeleteMeta_Call {
	_c.Call.Return(run)
	return _c
}

// GetMeta provides a mock function with given fields:
func (_m *MockProvider) GetMeta() map[string]string {
	ret := _m.Called()

	var r0 map[string]string
	if rf, ok := ret.Get(0).(func() map[string]string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]string)
		}
	}

	return r0
}

// MockProvider_GetMeta_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetMeta'
type MockProvider_GetMeta_Call struct {
	*mock.Call
}

// GetMeta is a helper method to define mock.On call
func (_e *MockProvider_Expecter) GetMeta() *MockProvider_GetMeta_Call {
	return &MockProvider_GetMeta_Call{Call: _e.mock.On("GetMeta")}
}

func (_c *MockProvider_GetMeta_Call) Run(run func()) *MockProvider_GetMeta_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockProvider_GetMeta_Call) Return(_a0 map[string]string) *MockProvider_GetMeta_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockProvider_GetMeta_Call) RunAndReturn(run func() map[string]string) *MockProvider_GetMeta_Call {
	_c.Call.Return(run)
	return _c
}

// LastError provides a mock function with given fields:
func (_m *MockProvider) LastError() (time.Time, error) {
	ret := _m.Called()

	var r0 time.Time
	var r1 error
	if rf, ok := ret.Get(0).(func() (time.Time, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() time.Time); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Time)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockProvider_LastError_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'LastError'
type MockProvider_LastError_Call struct {
	*mock.Call
}

// LastError is a helper method to define mock.On call
func (_e *MockProvider_Expecter) LastError() *MockProvider_LastError_Call {
	return &MockProvider_LastError_Call{Call: _e.mock.On("LastError")}
}

func (_c *MockProvider_LastError_Call) Run(run func()) *MockProvider_LastError_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockProvider_LastError_Call) Return(_a0 time.Time, _a1 error) *MockProvider_LastError_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockProvider_LastError_Call) RunAndReturn(run func() (time.Time, error)) *MockProvider_LastError_Call {
	_c.Call.Return(run)
	return _c
}

// Listen provides a mock function with given fields: capability
func (_m *MockProvider) Listen(capability string) (net.Listener, error) {
	ret := _m.Called(capability)

	var r0 net.Listener
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (net.Listener, error)); ok {
		return rf(capability)
	}
	if rf, ok := ret.Get(0).(func(string) net.Listener); ok {
		r0 = rf(capability)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(net.Listener)
		}
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(capability)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockProvider_Listen_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Listen'
type MockProvider_Listen_Call struct {
	*mock.Call
}

// Listen is a helper method to define mock.On call
//   - capability string
func (_e *MockProvider_Expecter) Listen(capability interface{}) *MockProvider_Listen_Call {
	return &MockProvider_Listen_Call{Call: _e.mock.On("Listen", capability)}
}

func (_c *MockProvider_Listen_Call) Run(run func(capability string)) *MockProvider_Listen_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *MockProvider_Listen_Call) Return(_a0 net.Listener, _a1 error) *MockProvider_Listen_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockProvider_Listen_Call) RunAndReturn(run func(string) (net.Listener, error)) *MockProvider_Listen_Call {
	_c.Call.Return(run)
	return _c
}

// SessionStatus provides a mock function with given fields:
func (_m *MockProvider) SessionStatus() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockProvider_SessionStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'SessionStatus'
type MockProvider_SessionStatus_Call struct {
	*mock.Call
}

// SessionStatus is a helper method to define mock.On call
func (_e *MockProvider_Expecter) SessionStatus() *MockProvider_SessionStatus_Call {
	return &MockProvider_SessionStatus_Call{Call: _e.mock.On("SessionStatus")}
}

func (_c *MockProvider_SessionStatus_Call) Run(run func()) *MockProvider_SessionStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockProvider_SessionStatus_Call) Return(_a0 string) *MockProvider_SessionStatus_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockProvider_SessionStatus_Call) RunAndReturn(run func() string) *MockProvider_SessionStatus_Call {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function with given fields:
func (_m *MockProvider) Start() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockProvider_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type MockProvider_Start_Call struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
func (_e *MockProvider_Expecter) Start() *MockProvider_Start_Call {
	return &MockProvider_Start_Call{Call: _e.mock.On("Start")}
}

func (_c *MockProvider_Start_Call) Run(run func()) *MockProvider_Start_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockProvider_Start_Call) Return(_a0 error) *MockProvider_Start_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockProvider_Start_Call) RunAndReturn(run func() error) *MockProvider_Start_Call {
	_c.Call.Return(run)
	return _c
}

// Stop provides a mock function with given fields:
func (_m *MockProvider) Stop() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockProvider_Stop_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Stop'
type MockProvider_Stop_Call struct {
	*mock.Call
}

// Stop is a helper method to define mock.On call
func (_e *MockProvider_Expecter) Stop() *MockProvider_Stop_Call {
	return &MockProvider_Stop_Call{Call: _e.mock.On("Stop")}
}

func (_c *MockProvider_Stop_Call) Run(run func()) *MockProvider_Stop_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockProvider_Stop_Call) Return(_a0 error) *MockProvider_Stop_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockProvider_Stop_Call) RunAndReturn(run func() error) *MockProvider_Stop_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateMeta provides a mock function with given fields: _a0
func (_m *MockProvider) UpdateMeta(_a0 map[string]string) {
	_m.Called(_a0)
}

// MockProvider_UpdateMeta_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateMeta'
type MockProvider_UpdateMeta_Call struct {
	*mock.Call
}

// UpdateMeta is a helper method to define mock.On call
//   - _a0 map[string]string
func (_e *MockProvider_Expecter) UpdateMeta(_a0 interface{}) *MockProvider_UpdateMeta_Call {
	return &MockProvider_UpdateMeta_Call{Call: _e.mock.On("UpdateMeta", _a0)}
}

func (_c *MockProvider_UpdateMeta_Call) Run(run func(_a0 map[string]string)) *MockProvider_UpdateMeta_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(map[string]string))
	})
	return _c
}

func (_c *MockProvider_UpdateMeta_Call) Return() *MockProvider_UpdateMeta_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockProvider_UpdateMeta_Call) RunAndReturn(run func(map[string]string)) *MockProvider_UpdateMeta_Call {
	_c.Call.Return(run)
	return _c
}

type mockConstructorTestingTNewMockProvider interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockProvider creates a new instance of MockProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockProvider(t mockConstructorTestingTNewMockProvider) *MockProvider {
	mock := &MockProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
