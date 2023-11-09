// Code generated by protoc-json-shim. DO NOT EDIT.
package meshv2beta1

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for ComputedProxyConfiguration
func (this *ComputedProxyConfiguration) MarshalJSON() ([]byte, error) {
	str, err := ComputedProxyConfigurationMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ComputedProxyConfiguration
func (this *ComputedProxyConfiguration) UnmarshalJSON(b []byte) error {
	return ComputedProxyConfigurationUnmarshaler.Unmarshal(b, this)
}

var (
	ComputedProxyConfigurationMarshaler   = &protojson.MarshalOptions{}
	ComputedProxyConfigurationUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
