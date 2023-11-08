// Code generated by protoc-json-shim. DO NOT EDIT.
package pbserverdiscovery

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for WatchServersRequest
func (this *WatchServersRequest) MarshalJSON() ([]byte, error) {
	str, err := ServerdiscoveryMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for WatchServersRequest
func (this *WatchServersRequest) UnmarshalJSON(b []byte) error {
	return ServerdiscoveryUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for WatchServersResponse
func (this *WatchServersResponse) MarshalJSON() ([]byte, error) {
	str, err := ServerdiscoveryMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for WatchServersResponse
func (this *WatchServersResponse) UnmarshalJSON(b []byte) error {
	return ServerdiscoveryUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for Server
func (this *Server) MarshalJSON() ([]byte, error) {
	str, err := ServerdiscoveryMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Server
func (this *Server) UnmarshalJSON(b []byte) error {
	return ServerdiscoveryUnmarshaler.Unmarshal(b, this)
}

var (
	ServerdiscoveryMarshaler   = &protojson.MarshalOptions{}
	ServerdiscoveryUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
