// Code generated by mockery v2.37.1. DO NOT EDIT.

package mockpbdataplane

import (
	context "context"

	pbdataplane "github.com/hashicorp/consul/proto-public/pbdataplane"
	mock "github.com/stretchr/testify/mock"
)

// DataplaneServiceServer is an autogenerated mock type for the DataplaneServiceServer type
type DataplaneServiceServer struct {
	mock.Mock
}

type DataplaneServiceServer_Expecter struct {
	mock *mock.Mock
}

func (_m *DataplaneServiceServer) EXPECT() *DataplaneServiceServer_Expecter {
	return &DataplaneServiceServer_Expecter{mock: &_m.Mock}
}

// GetEnvoyBootstrapParams provides a mock function with given fields: _a0, _a1
func (_m *DataplaneServiceServer) GetEnvoyBootstrapParams(_a0 context.Context, _a1 *pbdataplane.GetEnvoyBootstrapParamsRequest) (*pbdataplane.GetEnvoyBootstrapParamsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *pbdataplane.GetEnvoyBootstrapParamsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pbdataplane.GetEnvoyBootstrapParamsRequest) (*pbdataplane.GetEnvoyBootstrapParamsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pbdataplane.GetEnvoyBootstrapParamsRequest) *pbdataplane.GetEnvoyBootstrapParamsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbdataplane.GetEnvoyBootstrapParamsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pbdataplane.GetEnvoyBootstrapParamsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DataplaneServiceServer_GetEnvoyBootstrapParams_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetEnvoyBootstrapParams'
type DataplaneServiceServer_GetEnvoyBootstrapParams_Call struct {
	*mock.Call
}

// GetEnvoyBootstrapParams is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *pbdataplane.GetEnvoyBootstrapParamsRequest
func (_e *DataplaneServiceServer_Expecter) GetEnvoyBootstrapParams(_a0 interface{}, _a1 interface{}) *DataplaneServiceServer_GetEnvoyBootstrapParams_Call {
	return &DataplaneServiceServer_GetEnvoyBootstrapParams_Call{Call: _e.mock.On("GetEnvoyBootstrapParams", _a0, _a1)}
}

func (_c *DataplaneServiceServer_GetEnvoyBootstrapParams_Call) Run(run func(_a0 context.Context, _a1 *pbdataplane.GetEnvoyBootstrapParamsRequest)) *DataplaneServiceServer_GetEnvoyBootstrapParams_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*pbdataplane.GetEnvoyBootstrapParamsRequest))
	})
	return _c
}

func (_c *DataplaneServiceServer_GetEnvoyBootstrapParams_Call) Return(_a0 *pbdataplane.GetEnvoyBootstrapParamsResponse, _a1 error) *DataplaneServiceServer_GetEnvoyBootstrapParams_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *DataplaneServiceServer_GetEnvoyBootstrapParams_Call) RunAndReturn(run func(context.Context, *pbdataplane.GetEnvoyBootstrapParamsRequest) (*pbdataplane.GetEnvoyBootstrapParamsResponse, error)) *DataplaneServiceServer_GetEnvoyBootstrapParams_Call {
	_c.Call.Return(run)
	return _c
}

// GetSupportedDataplaneFeatures provides a mock function with given fields: _a0, _a1
func (_m *DataplaneServiceServer) GetSupportedDataplaneFeatures(_a0 context.Context, _a1 *pbdataplane.GetSupportedDataplaneFeaturesRequest) (*pbdataplane.GetSupportedDataplaneFeaturesResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *pbdataplane.GetSupportedDataplaneFeaturesResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *pbdataplane.GetSupportedDataplaneFeaturesRequest) (*pbdataplane.GetSupportedDataplaneFeaturesResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *pbdataplane.GetSupportedDataplaneFeaturesRequest) *pbdataplane.GetSupportedDataplaneFeaturesResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*pbdataplane.GetSupportedDataplaneFeaturesResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *pbdataplane.GetSupportedDataplaneFeaturesRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DataplaneServiceServer_GetSupportedDataplaneFeatures_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetSupportedDataplaneFeatures'
type DataplaneServiceServer_GetSupportedDataplaneFeatures_Call struct {
	*mock.Call
}

// GetSupportedDataplaneFeatures is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 *pbdataplane.GetSupportedDataplaneFeaturesRequest
func (_e *DataplaneServiceServer_Expecter) GetSupportedDataplaneFeatures(_a0 interface{}, _a1 interface{}) *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call {
	return &DataplaneServiceServer_GetSupportedDataplaneFeatures_Call{Call: _e.mock.On("GetSupportedDataplaneFeatures", _a0, _a1)}
}

func (_c *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call) Run(run func(_a0 context.Context, _a1 *pbdataplane.GetSupportedDataplaneFeaturesRequest)) *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*pbdataplane.GetSupportedDataplaneFeaturesRequest))
	})
	return _c
}

func (_c *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call) Return(_a0 *pbdataplane.GetSupportedDataplaneFeaturesResponse, _a1 error) *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call) RunAndReturn(run func(context.Context, *pbdataplane.GetSupportedDataplaneFeaturesRequest) (*pbdataplane.GetSupportedDataplaneFeaturesResponse, error)) *DataplaneServiceServer_GetSupportedDataplaneFeatures_Call {
	_c.Call.Return(run)
	return _c
}

// NewDataplaneServiceServer creates a new instance of DataplaneServiceServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDataplaneServiceServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *DataplaneServiceServer {
	mock := &DataplaneServiceServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
