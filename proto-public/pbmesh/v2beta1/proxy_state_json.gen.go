// Code generated by protoc-json-shim. DO NOT EDIT.
package meshv2beta1

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for ProxyStateTemplate
func (this *ProxyStateTemplate) MarshalJSON() ([]byte, error) {
	str, err := ProxyStateMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ProxyStateTemplate
func (this *ProxyStateTemplate) UnmarshalJSON(b []byte) error {
	return ProxyStateUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for ProxyState
func (this *ProxyState) MarshalJSON() ([]byte, error) {
	str, err := ProxyStateMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ProxyState
func (this *ProxyState) UnmarshalJSON(b []byte) error {
	return ProxyStateUnmarshaler.Unmarshal(b, this)
}

var (
	ProxyStateMarshaler   = &protojson.MarshalOptions{}
	ProxyStateUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
