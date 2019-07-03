// Package metadata provides an API that allows plugins to add metadata to the context.
// Each metadata is stored under a label that has the form <plugin>/<name>. Each metadata
// is returned as a Func. When Func is called the metadata is returned. If Func is expensive to
// execute it is its responsibility to provide some form of caching. During the handling of a
// query it is expected the metadata stays constant.
//
// Basic example:
//
// Implement the Provider interface for a plugin p:
//
//    func (p P) Metadata(ctx context.Context, state request.Request) context.Context {
//       metadata.SetValueFunc(ctx, "test/something", func() string { return "myvalue" })
//	 return ctx
//    }
//
// Basic example with caching:
//
//    func (p P) Metadata(ctx context.Context, state request.Request) context.Context {
//       cached := ""
//       f := func() string {
//		if cached != "" {
//                 return cached
//             }
//             cached = expensiveFunc()
//             return cached
//       }
//       metadata.SetValueFunc(ctx, "test/something", f)
//	 return ctx
//    }
//
// If you need access to this metadata from another plugin:
//
//    // ...
//    valueFunc := metadata.ValueFunc(ctx, "test/something")
//    value := valueFunc()
//    // use 'value'
//
package metadata

import (
	"context"
	"strings"

	"github.com/coredns/coredns/request"
)

// Provider interface needs to be implemented by each plugin willing to provide
// metadata information for other plugins.
type Provider interface {
	// Metadata adds metadata to the context and returns a (potentially) new context.
	// Note: this method should work quickly, because it is called for every request
	// from the metadata plugin.
	Metadata(ctx context.Context, state request.Request) context.Context
}

// Func is the type of function in the metadata, when called they return the value of the label.
type Func func() string

// IsLabel checks that the provided name is a valid label name, i.e. two words separated by a slash.
func IsLabel(label string) bool {
	p := strings.Index(label, "/")
	if p <= 0 || p >= len(label)-1 {
		// cannot accept namespace empty nor label empty
		return false
	}
	if strings.LastIndex(label, "/") != p {
		// several slash in the Label
		return false
	}
	return true

}

// Labels returns all metadata keys stored in the context. These label names should be named
// as: plugin/NAME, where NAME is something descriptive.
func Labels(ctx context.Context) []string {
	if metadata := ctx.Value(key{}); metadata != nil {
		if m, ok := metadata.(md); ok {
			return keys(m)
		}
	}
	return nil
}

// ValueFuncs returns the map[string]Func from the context, or nil if it does not exist.
func ValueFuncs(ctx context.Context) map[string]Func {
	if metadata := ctx.Value(key{}); metadata != nil {
		if m, ok := metadata.(md); ok {
			return m
		}
	}
	return nil
}

// ValueFunc returns the value function of label. If none can be found nil is returned. Calling the
// function returns the value of the label.
func ValueFunc(ctx context.Context, label string) Func {
	if metadata := ctx.Value(key{}); metadata != nil {
		if m, ok := metadata.(md); ok {
			return m[label]
		}
	}
	return nil
}

// SetValueFunc set the metadata label to the value function. If no metadata can be found this is a noop and
// false is returned. Any existing value is overwritten.
func SetValueFunc(ctx context.Context, label string, f Func) bool {
	if metadata := ctx.Value(key{}); metadata != nil {
		if m, ok := metadata.(md); ok {
			m[label] = f
			return true
		}
	}
	return false
}

// md is metadata information storage.
type md map[string]Func

// key defines the type of key that is used to save metadata into the context.
type key struct{}

func keys(m map[string]Func) []string {
	s := make([]string, len(m))
	i := 0
	for k := range m {
		s[i] = k
		i++
	}
	return s
}
