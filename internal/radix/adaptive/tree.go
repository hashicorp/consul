package adaptive

type RadixTree[T any] struct {
	root *Node[T]
	size uint64
}

func NewAdaptiveRadixTree[T any]() *RadixTree[T] {
	return &RadixTree[T]{root: nil, size: 0}
}
