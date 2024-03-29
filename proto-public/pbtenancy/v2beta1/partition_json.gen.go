// Code generated by protoc-json-shim. DO NOT EDIT.
package tenancyv2beta1

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for Partition
func (this *Partition) MarshalJSON() ([]byte, error) {
	str, err := PartitionMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Partition
func (this *Partition) UnmarshalJSON(b []byte) error {
	return PartitionUnmarshaler.Unmarshal(b, this)
}

var (
	PartitionMarshaler   = &protojson.MarshalOptions{}
	PartitionUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
