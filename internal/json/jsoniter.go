// +build jsoniter

package json

import "github.com/json-iterator/go"

type RawMessage = json.RawMessage

var (
	json = jsoniter.ConfigCompatibleWithStandardLibrary
	// Marshal is the counterpart to Marshal in github.com/json-iterator/go.
	Marshal = json.Marshal
	// Unmarshal is the counterpart to Unmarshal in github.com/json-iterator/go.
	Unmarshal = json.Unmarshal
	// NewEncoder is the counterpart to NewEncoder in github.com/json-iterator/go.
	NewEncoder = json.NewEncoder
	// NewDecoder is the counterpart to NewDecoder in github.com/json-iterator/go.
	NewDecoder = json.NewDecoder
	// MarshalIndent is the counterpart to MarshalIndent in github.com/json-iterator/go.
	MarshalIndent = json.MarshalIndent
	// Indent is the counterpart to Indent in github.com/json-iterator/go.
	Indent = json.Indent
)
