// Code generated by mockery v1.0.0. DO NOT EDIT.

package dataplane

import (
	acl "github.com/hashicorp/consul/acl"
	mock "github.com/stretchr/testify/mock"

	structs "github.com/hashicorp/consul/agent/structs"
)

// MockACLResolver is an autogenerated mock type for the ACLResolver type
type MockACLResolver struct {
	mock.Mock
}

// ResolveTokenAndDefaultMeta provides a mock function with given fields: _a0, _a1, _a2
func (_m *MockACLResolver) ResolveTokenAndDefaultMeta(_a0 string, _a1 *structs.EnterpriseMeta, _a2 *acl.AuthorizerContext) (acl.Authorizer, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 acl.Authorizer
	if rf, ok := ret.Get(0).(func(string, *structs.EnterpriseMeta, *acl.AuthorizerContext) acl.Authorizer); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(acl.Authorizer)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, *structs.EnterpriseMeta, *acl.AuthorizerContext) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
