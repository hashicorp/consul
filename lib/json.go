// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package lib

import (
	"bytes"
	"encoding/json"
	"io"
)

// DecodeJSON is a convenience function to create a JSON decoder
// set it up to disallow unknown fields and then decode into the
// given value
func DecodeJSON(data io.Reader, out interface{}) error {
	if data == nil {
		return io.EOF
	}

	decoder := json.NewDecoder(data)
	decoder.DisallowUnknownFields()
	return decoder.Decode(&out)
}

// UnmarshalJSON is a convenience function around calling
// DecodeJSON. It will mainly be useful in many of our
// UnmarshalJSON methods for structs.
func UnmarshalJSON(data []byte, out interface{}) error {
	return DecodeJSON(bytes.NewReader(data), out)
}
