// Code generated by protoc-gen-go-binary. DO NOT EDIT.
// source: private/pbstorage/raft.proto

package pbstorage

import (
	"google.golang.org/protobuf/proto"
)

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *Request) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *Request) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}
