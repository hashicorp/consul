// Code generated by mockery v2.41.0. DO NOT EDIT.

package mockpbserverdiscovery

import mock "github.com/stretchr/testify/mock"

// UnsafeServerDiscoveryServiceServer is an autogenerated mock type for the UnsafeServerDiscoveryServiceServer type
type UnsafeServerDiscoveryServiceServer struct {
	mock.Mock
}

type UnsafeServerDiscoveryServiceServer_Expecter struct {
	mock *mock.Mock
}

func (_m *UnsafeServerDiscoveryServiceServer) EXPECT() *UnsafeServerDiscoveryServiceServer_Expecter {
	return &UnsafeServerDiscoveryServiceServer_Expecter{mock: &_m.Mock}
}

// mustEmbedUnimplementedServerDiscoveryServiceServer provides a mock function with given fields:
func (_m *UnsafeServerDiscoveryServiceServer) mustEmbedUnimplementedServerDiscoveryServiceServer() {
	_m.Called()
}

// UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'mustEmbedUnimplementedServerDiscoveryServiceServer'
type UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call struct {
	*mock.Call
}

// mustEmbedUnimplementedServerDiscoveryServiceServer is a helper method to define mock.On call
func (_e *UnsafeServerDiscoveryServiceServer_Expecter) mustEmbedUnimplementedServerDiscoveryServiceServer() *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call {
	return &UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call{Call: _e.mock.On("mustEmbedUnimplementedServerDiscoveryServiceServer")}
}

func (_c *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call) Run(run func()) *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call) Return() *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call {
	_c.Call.Return()
	return _c
}

func (_c *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call) RunAndReturn(run func()) *UnsafeServerDiscoveryServiceServer_mustEmbedUnimplementedServerDiscoveryServiceServer_Call {
	_c.Call.Return(run)
	return _c
}

// NewUnsafeServerDiscoveryServiceServer creates a new instance of UnsafeServerDiscoveryServiceServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewUnsafeServerDiscoveryServiceServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *UnsafeServerDiscoveryServiceServer {
	mock := &UnsafeServerDiscoveryServiceServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
