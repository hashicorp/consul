// +build !jsoniter

package json

import "encoding/json"

type RawMessage = json.RawMessage

var (
	// Marshal is the counterpart to Marshal in encoding/json.
	Marshal = json.Marshal
	// Unmarshal is the counterpart to Unmarshal in encoding/json.
	Unmarshal = json.Unmarshal
	// NewEncoder is the counterpart to NewEncoder in encoding/json.
	NewEncoder = json.NewEncoder
	// NewDecoder is the counterpart to NewDecoder in encoding/json.
	NewDecoder = json.NewDecoder
	// MarshalIndent is the counterpart to MarshalIndent in encoding/json.
	MarshalIndent = json.MarshalIndent
	// Indent is the counterpart to Indent in encoding/json.
	Indent = json.Indent
)
