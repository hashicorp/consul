// Code generated by mockery v2.20.0. DO NOT EDIT.

package rate

import mock "github.com/stretchr/testify/mock"

// MockLeaderStatusProvider is an autogenerated mock type for the LeaderStatusProvider type
type MockLeaderStatusProvider struct {
	mock.Mock
}

// IsLeader provides a mock function with given fields:
func (_m *MockLeaderStatusProvider) IsLeader() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

type mockConstructorTestingTNewMockLeaderStatusProvider interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockLeaderStatusProvider creates a new instance of MockLeaderStatusProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockLeaderStatusProvider(t mockConstructorTestingTNewMockLeaderStatusProvider) *MockLeaderStatusProvider {
	mock := &MockLeaderStatusProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
