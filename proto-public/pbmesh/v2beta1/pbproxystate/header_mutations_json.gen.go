// Code generated by protoc-json-shim. DO NOT EDIT.
package pbproxystate

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for HeaderMutation
func (this *HeaderMutation) MarshalJSON() ([]byte, error) {
	str, err := HeaderMutationsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for HeaderMutation
func (this *HeaderMutation) UnmarshalJSON(b []byte) error {
	return HeaderMutationsUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for RequestHeaderAdd
func (this *RequestHeaderAdd) MarshalJSON() ([]byte, error) {
	str, err := HeaderMutationsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for RequestHeaderAdd
func (this *RequestHeaderAdd) UnmarshalJSON(b []byte) error {
	return HeaderMutationsUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for RequestHeaderRemove
func (this *RequestHeaderRemove) MarshalJSON() ([]byte, error) {
	str, err := HeaderMutationsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for RequestHeaderRemove
func (this *RequestHeaderRemove) UnmarshalJSON(b []byte) error {
	return HeaderMutationsUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for ResponseHeaderAdd
func (this *ResponseHeaderAdd) MarshalJSON() ([]byte, error) {
	str, err := HeaderMutationsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ResponseHeaderAdd
func (this *ResponseHeaderAdd) UnmarshalJSON(b []byte) error {
	return HeaderMutationsUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for ResponseHeaderRemove
func (this *ResponseHeaderRemove) MarshalJSON() ([]byte, error) {
	str, err := HeaderMutationsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ResponseHeaderRemove
func (this *ResponseHeaderRemove) UnmarshalJSON(b []byte) error {
	return HeaderMutationsUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for Header
func (this *Header) MarshalJSON() ([]byte, error) {
	str, err := HeaderMutationsMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for Header
func (this *Header) UnmarshalJSON(b []byte) error {
	return HeaderMutationsUnmarshaler.Unmarshal(b, this)
}

var (
	HeaderMutationsMarshaler   = &protojson.MarshalOptions{}
	HeaderMutationsUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
