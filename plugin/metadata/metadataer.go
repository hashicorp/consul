package metadata

import (
	"context"

	"github.com/miekg/dns"
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
	Metadata(context.Context, dns.ResponseWriter, *dns.Msg, string) (interface{}, bool)
}

// MD is metadata information storage
type MD map[string]interface{}

// metadataKey defines the type of key that is used to save metadata into the context
type metadataKey struct{}

// newMD initializes MD and attaches it to context
func newMD(ctx context.Context) (MD, context.Context) {
	m := MD{}
	return m, context.WithValue(ctx, metadataKey{}, m)
}

// FromContext retrieves MD struct from context.
func FromContext(ctx context.Context) (md MD, ok bool) {
	if metadata := ctx.Value(metadataKey{}); metadata != nil {
		if md, ok := metadata.(MD); ok {
			return md, true
		}
	}
	return MD{}, false
}

// Value returns metadata value by key.
func (m MD) Value(key string) (value interface{}, ok bool) {
	value, ok = m[key]
	return value, ok
}

// setValue adds metadata value.
func (m MD) setValue(key string, val interface{}) {
	m[key] = val
}
