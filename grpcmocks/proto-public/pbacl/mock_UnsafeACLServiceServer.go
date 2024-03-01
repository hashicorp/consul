// Code generated by mockery v2.41.0. DO NOT EDIT.

package mockpbacl

import mock "github.com/stretchr/testify/mock"

// UnsafeACLServiceServer is an autogenerated mock type for the UnsafeACLServiceServer type
type UnsafeACLServiceServer struct {
	mock.Mock
}

type UnsafeACLServiceServer_Expecter struct {
	mock *mock.Mock
}

func (_m *UnsafeACLServiceServer) EXPECT() *UnsafeACLServiceServer_Expecter {
	return &UnsafeACLServiceServer_Expecter{mock: &_m.Mock}
}

// mustEmbedUnimplementedACLServiceServer provides a mock function with given fields:
func (_m *UnsafeACLServiceServer) mustEmbedUnimplementedACLServiceServer() {
	_m.Called()
}

// UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'mustEmbedUnimplementedACLServiceServer'
type UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call struct {
	*mock.Call
}

// mustEmbedUnimplementedACLServiceServer is a helper method to define mock.On call
func (_e *UnsafeACLServiceServer_Expecter) mustEmbedUnimplementedACLServiceServer() *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call {
	return &UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call{Call: _e.mock.On("mustEmbedUnimplementedACLServiceServer")}
}

func (_c *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call) Run(run func()) *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call) Return() *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call {
	_c.Call.Return()
	return _c
}

func (_c *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call) RunAndReturn(run func()) *UnsafeACLServiceServer_mustEmbedUnimplementedACLServiceServer_Call {
	_c.Call.Return(run)
	return _c
}

// NewUnsafeACLServiceServer creates a new instance of UnsafeACLServiceServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewUnsafeACLServiceServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnsafeACLServiceServer {
	mock := &UnsafeACLServiceServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
