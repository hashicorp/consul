// Code generated by protoc-json-shim. DO NOT EDIT.
package multiclusterv2beta1

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for ExportedServices
func (this *ExportedServices) MarshalJSON() ([]byte, error) {
	str, err := ExportedServicesMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ExportedServices
func (this *ExportedServices) UnmarshalJSON(b []byte) error {
	return ExportedServicesUnmarshaler.Unmarshal(b, this)
}

var (
	ExportedServicesMarshaler   = &protojson.MarshalOptions{}
	ExportedServicesUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
