package adaptive

const MaxPrefixLen = 10
const LEAF = 0
const NODE4 = 1
const NODE16 = 2
const NODE48 = 3
const NODE256 = 4

type RadixTree[T any] struct {
	root *Node[T]
	size uint64
}

func NewAdaptiveRadixTree[T any]() *RadixTree[T] {
	return &RadixTree[T]{root: nil, size: 0}
}

func (t *RadixTree[T]) Insert(key []byte, value T) T {
	var old int
	oldVal := recursiveInsert[T](t.root, &t.root, getTreeKey(key), value, 0, &old)
	if old == 0 {
		t.size++
	}
	return oldVal
}

func (t *RadixTree[T]) Search(key []byte) T {
	return iterativeSearch[T](t, getTreeKey(key))
}

func (t *RadixTree[T]) Minimum() *NodeLeaf[T] {
	return minimum[T](*t.root)
}

func (t *RadixTree[T]) Maximum() *NodeLeaf[T] {
	return maximum[T](*t.root)
}
