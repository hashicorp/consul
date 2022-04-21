// Code generated by mockery v2.11.0. DO NOT EDIT.

package connectca

import (
	acl "github.com/hashicorp/consul/acl"
	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// MockACLResolver is an autogenerated mock type for the ACLResolver type
type MockACLResolver struct {
	mock.Mock
}

// ResolveTokenAndDefaultMeta provides a mock function with given fields: token, entMeta, authzContext
func (_m *MockACLResolver) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzContext *acl.AuthorizerContext) (acl.Authorizer, error) {
	ret := _m.Called(token, entMeta, authzContext)

	var r0 acl.Authorizer
	if rf, ok := ret.Get(0).(func(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) acl.Authorizer); ok {
		r0 = rf(token, entMeta, authzContext)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(acl.Authorizer)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) error); ok {
		r1 = rf(token, entMeta, authzContext)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMockACLResolver creates a new instance of MockACLResolver. It also registers a cleanup function to assert the mocks expectations.
func NewMockACLResolver(t testing.TB) *MockACLResolver {
	mock := &MockACLResolver{}

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
