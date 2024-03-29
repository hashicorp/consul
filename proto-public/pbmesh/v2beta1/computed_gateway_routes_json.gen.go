// Code generated by protoc-json-shim. DO NOT EDIT.
package meshv2beta1

import (
	protojson "google.golang.org/protobuf/encoding/protojson"
)

// MarshalJSON is a custom marshaler for ComputedGatewayRoutes
func (this *ComputedGatewayRoutes) MarshalJSON() ([]byte, error) {
	str, err := ComputedGatewayRoutesMarshaler.Marshal(this)
	return []byte(str), err
}

// UnmarshalJSON is a custom unmarshaler for ComputedGatewayRoutes
func (this *ComputedGatewayRoutes) UnmarshalJSON(b []byte) error {
	return ComputedGatewayRoutesUnmarshaler.Unmarshal(b, this)
}

var (
	ComputedGatewayRoutesMarshaler   = &protojson.MarshalOptions{}
	ComputedGatewayRoutesUnmarshaler = &protojson.UnmarshalOptions{DiscardUnknown: false}
)
