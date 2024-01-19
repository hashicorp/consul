// Code generated by protoc-gen-grpc-inmem. DO NOT EDIT.

package pbresource

import (
	grpc "google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type serverStream[T proto.Message] interface {
	Recv() (T, error)
	grpc.ClientStream
}

type cloningStream[T proto.Message] struct {
	serverStream[T]
}

func newCloningStream[T proto.Message](stream serverStream[T]) cloningStream[T] {
	return cloningStream[T]{serverStream: stream}
}

func (st cloningStream[T]) Recv() (T, error) {
	var zero T
	val, err := st.serverStream.Recv()
	if err != nil {
		return zero, err
	}

	return proto.Clone(val).(T), nil
}
