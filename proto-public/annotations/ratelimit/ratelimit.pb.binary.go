// Code generated by protoc-gen-go-binary. DO NOT EDIT.
// source: annotations/ratelimit/ratelimit.proto

package ratelimit

import (
	"google.golang.org/protobuf/proto"
)

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *Spec) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *Spec) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}
