package adaptive

type Node[T any] interface {
	getPartialLen() uint32
	setPartialLen(uint32)
	getArtNodeType() uint8
	setArtNodeType(uint8)
	getNumChildren() uint8
	setNumChildren(uint8)
	getPartial() []byte
	setPartial([]byte)
	isLeaf() bool
}
