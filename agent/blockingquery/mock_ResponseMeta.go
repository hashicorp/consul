// Code generated by mockery v2.32.4. DO NOT EDIT.

package blockingquery

import (
	time "time"

	mock "github.com/stretchr/testify/mock"
)

// MockResponseMeta is an autogenerated mock type for the ResponseMeta type
type MockResponseMeta struct {
	mock.Mock
}

// GetIndex provides a mock function with given fields:
func (_m *MockResponseMeta) GetIndex() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// SetIndex provides a mock function with given fields: _a0
func (_m *MockResponseMeta) SetIndex(_a0 uint64) {
	_m.Called(_a0)
}

// SetKnownLeader provides a mock function with given fields: _a0
func (_m *MockResponseMeta) SetKnownLeader(_a0 bool) {
	_m.Called(_a0)
}

// SetLastContact provides a mock function with given fields: _a0
func (_m *MockResponseMeta) SetLastContact(_a0 time.Duration) {
	_m.Called(_a0)
}

// SetResultsFilteredByACLs provides a mock function with given fields: _a0
func (_m *MockResponseMeta) SetResultsFilteredByACLs(_a0 bool) {
	_m.Called(_a0)
}

// NewMockResponseMeta creates a new instance of MockResponseMeta. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockResponseMeta(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockResponseMeta {
	mock := &MockResponseMeta{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
