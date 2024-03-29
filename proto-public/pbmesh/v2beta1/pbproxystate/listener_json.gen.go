// Code generated by protoc-json-shim. DO NOT EDIT.
package pbproxystate

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for Listener
func (this *Listener) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Listener
func (this *Listener) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for Router
func (this *Router) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Router
func (this *Router) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for Match
func (this *Match) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Match
func (this *Match) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for CidrRange
func (this *CidrRange) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for CidrRange
func (this *CidrRange) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for L4Destination
func (this *L4Destination) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for L4Destination
func (this *L4Destination) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for L7DestinationRoute
func (this *L7DestinationRoute) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for L7DestinationRoute
func (this *L7DestinationRoute) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for L7Destination
func (this *L7Destination) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for L7Destination
func (this *L7Destination) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for SNIDestination
func (this *SNIDestination) MarshalJSON() ([]byte, error) {
	str, err := ListenerMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for SNIDestination
func (this *SNIDestination) UnmarshalJSON(b []byte) error {
	return ListenerUnmarshaler.Unmarshal(b, this)
}

var (
	ListenerMarshaler   = &protojson.MarshalOptions{}
	ListenerUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
