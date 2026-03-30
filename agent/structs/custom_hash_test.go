// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package structs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testHashAppender struct {
	text string
}

func (a testHashAppender) appendHash(h *customHasher) {
	h.addString(a.text)
}

type testHashValue struct {
	hash uint64
}

func (v *testHashValue) getHash() uint64 {
	return v.hash
}

func TestHashValue_UsesAppenderOutput(t *testing.T) {
	require.Equal(t, hashValue(testHashAppender{text: "alpha"}), hashValue(testHashAppender{text: "alpha"}))
	require.NotEqual(t, hashValue(testHashAppender{text: "alpha"}), hashValue(testHashAppender{text: "beta"}))
}

func TestCustomHasherAddString_DelimitsAdjacentValues(t *testing.T) {
	joined := newCustomHasher().addString("ab").addString("c").Sum64()
	split := newCustomHasher().addString("a").addString("bc").Sum64()

	require.NotEqual(t, joined, split)
}

func TestCustomHasherAddJSONValue_ErrorFallsBackToZero(t *testing.T) {
	fromJSONError := newCustomHasher().addJSONValue(func() {}).Sum64()
	fromZero := newCustomHasher().addUint64(0).Sum64()

	require.Equal(t, fromZero, fromJSONError)
}

func TestAddOptionalValue_TracksPresenceAndHash(t *testing.T) {
	noValue := newCustomHasher()
	addOptionalValue[*testHashValue](noValue, nil)

	withHashOne := newCustomHasher()
	addOptionalValue(withHashOne, &testHashValue{hash: 1})

	withHashOneAgain := newCustomHasher()
	addOptionalValue(withHashOneAgain, &testHashValue{hash: 1})

	withHashTwo := newCustomHasher()
	addOptionalValue(withHashTwo, &testHashValue{hash: 2})

	require.NotEqual(t, noValue.Sum64(), withHashOne.Sum64())
	require.Equal(t, withHashOne.Sum64(), withHashOneAgain.Sum64())
	require.NotEqual(t, withHashOne.Sum64(), withHashTwo.Sum64())
}

func TestAddSlice_TracksLengthAndOrder(t *testing.T) {
	forward := newCustomHasher()
	addSlice(forward, []int{1, 2}, func(h *customHasher, value int) {
		h.addInt64(int64(value))
	})

	reversed := newCustomHasher()
	addSlice(reversed, []int{2, 1}, func(h *customHasher, value int) {
		h.addInt64(int64(value))
	})

	shorter := newCustomHasher()
	addSlice(shorter, []int{1}, func(h *customHasher, value int) {
		h.addInt64(int64(value))
	})

	require.NotEqual(t, forward.Sum64(), reversed.Sum64())
	require.NotEqual(t, forward.Sum64(), shorter.Sum64())
}

func TestAddOptionalValueSlice_TracksValues(t *testing.T) {
	orderedA := newCustomHasher()
	addOptionalValueSlice(orderedA, []*testHashValue{{hash: 1}, nil, {hash: 2}})

	orderedB := newCustomHasher()
	addOptionalValueSlice(orderedB, []*testHashValue{{hash: 1}, nil, {hash: 2}})

	reordered := newCustomHasher()
	addOptionalValueSlice(reordered, []*testHashValue{{hash: 2}, nil, {hash: 1}})

	require.Equal(t, orderedA.Sum64(), orderedB.Sum64())
	require.NotEqual(t, orderedA.Sum64(), reordered.Sum64())
}

func TestAddSortedStringKeyMap_IsOrderIndependent(t *testing.T) {
	mapA := newCustomHasher()
	addSortedStringKeyMap(mapA, map[string]int{
		"b": 2,
		"a": 1,
	}, func(h *customHasher, key string, value int) {
		h.addString(key).addInt64(int64(value))
	})

	mapB := newCustomHasher()
	addSortedStringKeyMap(mapB, map[string]int{
		"a": 1,
		"b": 2,
	}, func(h *customHasher, key string, value int) {
		h.addString(key).addInt64(int64(value))
	})

	changed := newCustomHasher()
	addSortedStringKeyMap(changed, map[string]int{
		"a": 1,
		"b": 3,
	}, func(h *customHasher, key string, value int) {
		h.addString(key).addInt64(int64(value))
	})

	require.Equal(t, mapA.Sum64(), mapB.Sum64())
	require.NotEqual(t, mapA.Sum64(), changed.Sum64())
}

func TestAddSortedStringKeyOptionalValueMap_IsOrderIndependent(t *testing.T) {
	mapA := newCustomHasher()
	addSortedStringKeyOptionalValueMap(mapA, map[string]*testHashValue{
		"b": {hash: 2},
		"a": nil,
	})

	mapB := newCustomHasher()
	addSortedStringKeyOptionalValueMap(mapB, map[string]*testHashValue{
		"a": nil,
		"b": {hash: 2},
	})

	changed := newCustomHasher()
	addSortedStringKeyOptionalValueMap(changed, map[string]*testHashValue{
		"a": nil,
		"b": {hash: 3},
	})

	require.Equal(t, mapA.Sum64(), mapB.Sum64())
	require.NotEqual(t, mapA.Sum64(), changed.Sum64())
}
