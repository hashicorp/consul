// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"encoding/binary"
	"encoding/json"
	"hash"
	"hash/fnv"
	"math"
	"sort"
	"time"
)

type hashAppender interface {
	appendHash(*customHasher)
}

type hashValueProvider interface {
	getHash() uint64
}

type customHasher struct {
	h   hash.Hash64
	buf [8]byte
}

func newCustomHasher() *customHasher {
	return &customHasher{h: fnv.New64a()}
}

func hashValue(v hashAppender) uint64 {
	h := newCustomHasher()
	v.appendHash(h)
	return h.Sum64()
}

func (h *customHasher) Sum64() uint64 {
	return h.h.Sum64()
}

func (h *customHasher) addByte(v byte) *customHasher {
	h.buf[0] = v
	_, _ = h.h.Write(h.buf[:1])
	return h
}

func (h *customHasher) addBool(v bool) *customHasher {
	if v {
		return h.addByte(1)
	}
	return h.addByte(0)
}

func (h *customHasher) addUint64(v uint64) *customHasher {
	binary.LittleEndian.PutUint64(h.buf[:], v)
	_, _ = h.h.Write(h.buf[:])
	return h
}

func (h *customHasher) addInt64(v int64) *customHasher {
	return h.addUint64(uint64(v))
}

func (h *customHasher) addDuration(v time.Duration) *customHasher {
	return h.addInt64(int64(v))
}

func (h *customHasher) addFloat32(v float32) *customHasher {
	return h.addUint64(uint64(math.Float32bits(v)))
}

func (h *customHasher) addString(v string) *customHasher {
	_, _ = h.h.Write([]byte(v))
	return h.addByte(0)
}

func (h *customHasher) addJSONValue(v any) *customHasher {
	encoded, err := json.Marshal(v)
	if err != nil {
		return h.addUint64(0)
	}
	return h.addString(string(encoded))
}

func addOptionalValue[T interface {
	hashValueProvider
	comparable
}](h *customHasher, value T) *customHasher {
	var zero T
	hasValue := value != zero
	h.addBool(hasValue)
	if hasValue {
		h.addUint64(value.getHash())
	}
	return h
}

func addSlice[T any](h *customHasher, values []T, addValue func(*customHasher, T)) *customHasher {
	h.addUint64(uint64(len(values)))
	for _, value := range values {
		addValue(h, value)
	}
	return h
}

func addOptionalValueSlice[T interface {
	hashValueProvider
	comparable
}](h *customHasher, values []T) *customHasher {
	return addSlice(h, values, func(h *customHasher, value T) {
		addOptionalValue(h, value)
	})
}

func addSortedStringKeyMap[T any](h *customHasher, values map[string]T, addValue func(*customHasher, string, T)) *customHasher {
	h.addUint64(uint64(len(values)))

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		addValue(h, key, values[key])
	}
	return h
}

func addSortedStringKeyOptionalValueMap[T interface {
	hashValueProvider
	comparable
}](h *customHasher, values map[string]T) *customHasher {
	return addSortedStringKeyMap(h, values, func(h *customHasher, key string, value T) {
		h.addString(key)
		addOptionalValue(h, value)
	})
}
