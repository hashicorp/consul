// Code generated by protoc-json-shim. DO NOT EDIT.
package pbproxystate

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for LeafCertificateRef
func (this *LeafCertificateRef) MarshalJSON() ([]byte, error) {
	str, err := ReferencesMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for LeafCertificateRef
func (this *LeafCertificateRef) UnmarshalJSON(b []byte) error {
	return ReferencesUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for TrustBundleRef
func (this *TrustBundleRef) MarshalJSON() ([]byte, error) {
	str, err := ReferencesMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for TrustBundleRef
func (this *TrustBundleRef) UnmarshalJSON(b []byte) error {
	return ReferencesUnmarshaler.Unmarshal(b, this)
}

// MarshalJSON is a custom marshaler for EndpointRef
func (this *EndpointRef) MarshalJSON() ([]byte, error) {
	str, err := ReferencesMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for EndpointRef
func (this *EndpointRef) UnmarshalJSON(b []byte) error {
	return ReferencesUnmarshaler.Unmarshal(b, this)
}

var (
	ReferencesMarshaler   = &protojson.MarshalOptions{}
	ReferencesUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
