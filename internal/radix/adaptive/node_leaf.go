package adaptive

type NodeLeaf[T any] struct {
	value       T
	keyLen      uint32
	key         []byte
	artNodeType uint8
}

func (n *NodeLeaf[T]) getPartialLen() uint32 {
	// no-op
	return 0
}

func (n *NodeLeaf[T]) setPartialLen(partialLen uint32) {
	// no-op
}

func (n *NodeLeaf[T]) getArtNodeType() uint8 {
	return n.artNodeType
}

func (n *NodeLeaf[T]) setArtNodeType(artNodeType uint8) {
	n.artNodeType = artNodeType
}

func (n *NodeLeaf[T]) getNumChildren() uint8 {
	return 0
}

func (n *NodeLeaf[T]) setNumChildren(numChildren uint8) {
	// no-op
}

func (n *NodeLeaf[T]) isLeaf() bool {
	return true
}

func (n *NodeLeaf[T]) getValue() interface{} {
	return n.value
}

func (n *NodeLeaf[T]) setValue(value T) {
	n.value = value
}

func (n *NodeLeaf[T]) getKeyLen() uint32 {
	return n.keyLen
}

func (n *NodeLeaf[T]) setKeyLen(keyLen uint32) {
	n.keyLen = keyLen
}

func (n *NodeLeaf[T]) getKey() []byte {
	return n.key
}

func (n *NodeLeaf[T]) setKey(key []byte) {
	n.key = key
}

func (n *NodeLeaf[T]) getPartial() []byte {
	//no-op
	return []byte{}
}

func (n *NodeLeaf[T]) setPartial(partial []byte) {
	// no-op
}
