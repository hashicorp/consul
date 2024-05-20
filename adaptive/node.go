// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package adaptive

type Node[T any] interface {
	getPartialLen() uint32
	setPartialLen(uint32)
	getArtNodeType() nodeType
	getNumChildren() uint8
	setNumChildren(uint8)
	getPartial() []byte
	setPartial([]byte)
	isLeaf() bool
	matchPrefix([]byte) bool
	getChild(int) Node[T]
	setChild(int, Node[T])
	clone() Node[T]
	getKey() []byte
	getValue() T
	setValue(T)
	setKey([]byte)
	getKeyLen() uint32
	setKeyLen(uint32)
	getKeyAtIdx(int) byte
	setKeyAtIdx(int, byte)
	getChildren() []Node[T]
	getKeys() []byte
	getMutateCh() chan struct{}
	setMutateCh(chan struct{})

	Iterator() *Iterator[T]
	PathIterator([]byte) *PathIterator[T]
}
