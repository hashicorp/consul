// Code generated by protoc-json-shim. DO NOT EDIT.
package multiclusterv2

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for PartitionExportedServices
func (this *PartitionExportedServices) MarshalJSON() ([]byte, error) {
	str, err := PartitionExportedServicesMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for PartitionExportedServices
func (this *PartitionExportedServices) UnmarshalJSON(b []byte) error {
	return PartitionExportedServicesUnmarshaler.Unmarshal(b, this)
}

var (
	PartitionExportedServicesMarshaler   = &protojson.MarshalOptions{}
	PartitionExportedServicesUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
