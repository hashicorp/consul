// Code generated by protoc-gen-go-binary. DO NOT EDIT.
// source: pbmulticluster/v1alpha1/sameness_group.proto

package multiclusterv1alpha1

import (
	"google.golang.org/protobuf/proto"
)

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *SamenessGroup) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *SamenessGroup) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}

// MarshalBinary implements encoding.BinaryMarshaler
func (msg *SamenessGroupMember) MarshalBinary() ([]byte, error) {
	return proto.Marshal(msg)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (msg *SamenessGroupMember) UnmarshalBinary(b []byte) error {
	return proto.Unmarshal(b, msg)
}
