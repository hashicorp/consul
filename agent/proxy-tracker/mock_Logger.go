// Code generated by mockery v2.32.4. DO NOT EDIT.

package proxytracker

import mock "github.com/stretchr/testify/mock"

// MockLogger is an autogenerated mock type for the Logger type
type MockLogger struct {
	mock.Mock
}

// Error provides a mock function with given fields: args
func (_m *MockLogger) Error(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// NewMockLogger creates a new instance of MockLogger. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockLogger {
	mock := &MockLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
