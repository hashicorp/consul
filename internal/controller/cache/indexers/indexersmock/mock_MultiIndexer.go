// Code generated by mockery v2.37.1. DO NOT EDIT.

package indexersmock

import (
	resource "github.com/hashicorp/consul/internal/resource"
	mock "github.com/stretchr/testify/mock"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// MultiIndexer is an autogenerated mock type for the MultiIndexer type
type MultiIndexer[T protoreflect.ProtoMessage] struct {
	mock.Mock
}

type MultiIndexer_Expecter[T protoreflect.ProtoMessage] struct {
	mock *mock.Mock
}

func (_m *MultiIndexer[T]) EXPECT() *MultiIndexer_Expecter[T] {
	return &MultiIndexer_Expecter[T]{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: r
func (_m *MultiIndexer[T]) Execute(r *resource.DecodedResource[T]) (bool, [][]byte, error) {
	ret := _m.Called(r)

	var r0 bool
	var r1 [][]byte
	var r2 error
	if rf, ok := ret.Get(0).(func(*resource.DecodedResource[T]) (bool, [][]byte, error)); ok {
		return rf(r)
	}
	if rf, ok := ret.Get(0).(func(*resource.DecodedResource[T]) bool); ok {
		r0 = rf(r)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(*resource.DecodedResource[T]) [][]byte); ok {
		r1 = rf(r)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([][]byte)
		}
	}

	if rf, ok := ret.Get(2).(func(*resource.DecodedResource[T]) error); ok {
		r2 = rf(r)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MultiIndexer_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type MultiIndexer_Execute_Call[T protoreflect.ProtoMessage] struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - r *resource.DecodedResource[T]
func (_e *MultiIndexer_Expecter[T]) Execute(r interface{}) *MultiIndexer_Execute_Call[T] {
	return &MultiIndexer_Execute_Call[T]{Call: _e.mock.On("Execute", r)}
}

func (_c *MultiIndexer_Execute_Call[T]) Run(run func(r *resource.DecodedResource[T])) *MultiIndexer_Execute_Call[T] {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*resource.DecodedResource[T]))
	})
	return _c
}

func (_c *MultiIndexer_Execute_Call[T]) Return(_a0 bool, _a1 [][]byte, _a2 error) *MultiIndexer_Execute_Call[T] {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MultiIndexer_Execute_Call[T]) RunAndReturn(run func(*resource.DecodedResource[T]) (bool, [][]byte, error)) *MultiIndexer_Execute_Call[T] {
	_c.Call.Return(run)
	return _c
}

// NewMultiIndexer creates a new instance of MultiIndexer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMultiIndexer[T protoreflect.ProtoMessage](t interface {
	mock.TestingT
	Cleanup(func())
}) *MultiIndexer[T] {
	mock := &MultiIndexer[T]{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
