// Code generated by protoc-json-shim. DO NOT EDIT.
package catalogv2beta1

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for ServiceEndpoints
func (this *ServiceEndpoints) MarshalJSON() ([]byte, error) {
	str, err := ServiceEndpointsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ServiceEndpoints
func (this *ServiceEndpoints) UnmarshalJSON(b []byte) error {
	return ServiceEndpointsUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for Endpoint
func (this *Endpoint) MarshalJSON() ([]byte, error) {
	str, err := ServiceEndpointsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Endpoint
func (this *Endpoint) UnmarshalJSON(b []byte) error {
	return ServiceEndpointsUnmarshaler.Unmarshal(b, this)
}

var (
	ServiceEndpointsMarshaler   = &protojson.MarshalOptions{}
	ServiceEndpointsUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
