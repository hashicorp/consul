// Code generated by mockery v2.15.0. DO NOT EDIT.

package cachetype

import (
	context "context"

	grpc "google.golang.org/grpc"

	mock "github.com/stretchr/testify/mock"

	pbpeering "github.com/hashicorp/consul/proto/private/pbpeering"
)

// MockTrustBundleReader is an autogenerated mock type for the TrustBundleReader type
type MockTrustBundleReader struct {
	mock.Mock
}

// TrustBundleRead provides a mock function with given fields: ctx, in, opts
func (_m *MockTrustBundleReader) TrustBundleRead(ctx context.Context, in *pbpeering.TrustBundleReadRequest, opts ...grpc.CallOption) (*pbpeering.TrustBundleReadResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *pbpeering.TrustBundleReadResponse
	if rf, ok := ret.Get(0).(func(context.Context, *pbpeering.TrustBundleReadRequest, ...grpc.CallOption) *pbpeering.TrustBundleReadResponse); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbpeering.TrustBundleReadResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *pbpeering.TrustBundleReadRequest, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewMockTrustBundleReader interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockTrustBundleReader creates a new instance of MockTrustBundleReader. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockTrustBundleReader(t mockConstructorTestingTNewMockTrustBundleReader) *MockTrustBundleReader {
	mock := &MockTrustBundleReader{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
