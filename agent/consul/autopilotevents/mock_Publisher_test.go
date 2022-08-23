// Code generated by mockery v2.12.2. DO NOT EDIT.

package autopilotevents

import (
	testing "testing"

	stream "github.com/hashicorp/consul/agent/consul/stream"
	mock "github.com/stretchr/testify/mock"
)

// MockPublisher is an autogenerated mock type for the Publisher type
type MockPublisher struct {
	mock.Mock
}

// Publish provides a mock function with given fields: _a0
func (_m *MockPublisher) Publish(_a0 []stream.Event) {
	_m.Called(_a0)
}

// NewMockPublisher creates a new instance of MockPublisher. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockPublisher(t testing.TB) *MockPublisher {
	mock := &MockPublisher{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
