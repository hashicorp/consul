// Code generated by protoc-gen-go-binary. DO NOT EDIT.
// source: private/pbstorage/raft.proto

package pbstorage

import (
	"google.golang.org/protobuf/proto"
)

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *Log) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *Log) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *LogResponse) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *LogResponse) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *WriteRequest) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *WriteRequest) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *WriteResponse) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *WriteResponse) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *DeleteRequest) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *DeleteRequest) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *ReadRequest) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *ReadRequest) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *ReadResponse) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *ReadResponse) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *ListRequest) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *ListRequest) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *ListResponse) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *ListResponse) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *GroupVersionMismatchErrorDetails) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *GroupVersionMismatchErrorDetails) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}
