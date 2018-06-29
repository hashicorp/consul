package metadata

import (
	"context"

	"github.com/coredns/coredns/request"
)

// Provider interface needs to be implemented by each plugin willing to provide
// metadata information for other plugins.
// Note: this method should work quickly, because it is called for every request
// from the metadata plugin.
type Provider interface {
	// List of variables which are provided by current Provider. Must remain constant.
	MetadataVarNames() []string
	// Metadata is expected to return a value with metadata information by the key
	// from 4th argument. Value can be later retrieved from context by any other plugin.
	// If value is not available by some reason returned boolean value should be false.
	Metadata(ctx context.Context, state request.Request, variable string) (interface{}, bool)
}

// M is metadata information storage.
type M map[string]interface{}

// FromContext retrieves the metadata from the context.
func FromContext(ctx context.Context) (M, bool) {
	if metadata := ctx.Value(metadataKey{}); metadata != nil {
		if m, ok := metadata.(M); ok {
			return m, true
		}
	}
	return M{}, false
}

// Value returns metadata value by key.
func (m M) Value(key string) (value interface{}, ok bool) {
	value, ok = m[key]
	return value, ok
}

// SetValue sets the metadata value under key.
func (m M) SetValue(key string, val interface{}) {
	m[key] = val
}

// metadataKey defines the type of key that is used to save metadata into the context.
type metadataKey struct{}
