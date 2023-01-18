// Code generated by mockery v2.15.0. DO NOT EDIT.

package connectca

import (
	acl "github.com/hashicorp/consul/acl"
	mock "github.com/stretchr/testify/mock"

	resolver "github.com/hashicorp/consul/acl/resolver"
)

// MockACLResolver is an autogenerated mock type for the ACLResolver type
type MockACLResolver struct {
	mock.Mock
}

// ResolveTokenAndDefaultMeta provides a mock function with given fields: token, entMeta, authzContext
func (_m *MockACLResolver) ResolveTokenAndDefaultMeta(token string, entMeta *acl.EnterpriseMeta, authzContext *acl.AuthorizerContext) (resolver.Result, error) {
	ret := _m.Called(token, entMeta, authzContext)

	var r0 resolver.Result
	if rf, ok := ret.Get(0).(func(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) resolver.Result); ok {
		r0 = rf(token, entMeta, authzContext)
	} else {
		r0 = ret.Get(0).(resolver.Result)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, *acl.EnterpriseMeta, *acl.AuthorizerContext) error); ok {
		r1 = rf(token, entMeta, authzContext)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockACLResolver interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockACLResolver creates a new instance of MockACLResolver. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockACLResolver(t mockConstructorTestingTNewMockACLResolver) *MockACLResolver {
	mock := &MockACLResolver{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
