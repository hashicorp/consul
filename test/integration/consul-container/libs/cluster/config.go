// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package cluster

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mitchellh/copystructure"
)

// TODO: rename file

type ConfigBuilder struct {
	nodes map[string]any
}

var _ json.Marshaler = (*ConfigBuilder)(nil)

func (b *ConfigBuilder) Clone() (*ConfigBuilder, error) {
	if b.nodes == nil {
		return &ConfigBuilder{}, nil
	}

	raw, err := copystructure.Copy(b.nodes)
	if err != nil {
		return nil, err
	}
	return &ConfigBuilder{
		nodes: raw.(map[string]any),
	}, nil
}

func (b *ConfigBuilder) MarshalJSON() ([]byte, error) {
	if b == nil || len(b.nodes) == 0 {
		return []byte("{}"), nil
	}

	return json.Marshal(b.nodes)
}

func (b *ConfigBuilder) String() string {
	d, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return "<ERR: " + err.Error() + ">"
	}
	return string(d)
}

func (b *ConfigBuilder) GetString(k string) (string, bool) {
	raw, ok := b.Get(k)
	if !ok {
		return "", false
	}

	return raw.(string), true
}

func (b *ConfigBuilder) GetBool(k string) (bool, bool) {
	raw, ok := b.Get(k)
	if !ok {
		return false, false
	}

	return raw.(bool), true
}

func (b *ConfigBuilder) Get(k string) (any, bool) {
	if b.nodes == nil {
		return nil, false
	}

	parts := strings.Split(k, ".")

	switch len(parts) {
	case 0:
		return nil, false
	case 1:
		v, ok := b.nodes[k]
		return v, ok
	}

	parents, child := parts[0:len(parts)-1], parts[len(parts)-1]

	curr := b.nodes
	for _, parent := range parents {
		next, ok := curr[parent]
		if !ok {
			return nil, false
		}
		curr = next.(map[string]any)
	}

	v, ok := curr[child]
	return v, ok
}

func (b *ConfigBuilder) Set(k string, v any) {
	if b.nodes == nil {
		b.nodes = make(map[string]any)
	}

	validateValueType(v)

	parts := strings.Split(k, ".")

	switch len(parts) {
	case 0:
		return
	case 1:
		b.nodes[k] = v
		return
	}

	parents, child := parts[0:len(parts)-1], parts[len(parts)-1]

	curr := b.nodes
	for _, parent := range parents {
		next, ok := curr[parent]
		if ok {
			curr = next.(map[string]any)
		} else {
			next := make(map[string]any)
			curr[parent] = next
			curr = next
		}
	}

	curr[child] = v
}

func validateValueType(v any) {
	switch x := v.(type) {
	case string:
	case int:
	case bool:
	case []string:
	case []any:
		for _, item := range x {
			validateSliceValueType(item)
		}
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

func validateSliceValueType(v any) {
	switch v.(type) {
	case string:
	case int:
	case bool:
	case *ConfigBuilder:
	default:
		panic(fmt.Sprintf("unexpected type %T", v))
	}
}

func (b *ConfigBuilder) Unset(k string) {
	if b.nodes == nil {
		return
	}

	parts := strings.Split(k, ".")

	switch len(parts) {
	case 0:
		return
	case 1:
		delete(b.nodes, k)
		return
	}

	parents, child := parts[0:len(parts)-1], parts[len(parts)-1]

	curr := b.nodes
	for _, parent := range parents {
		next, ok := curr[parent]
		if !ok {
			return
		}
		curr = next.(map[string]any)
	}

	delete(curr, child)
}
