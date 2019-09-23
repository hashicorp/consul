// Copyright ©2012 The bíogo Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found at the end of this file.

// Package tree implements Left-Leaning Red Black trees as described by Robert Sedgewick.
//
// More details relating to the implementation are available at the following locations:
//
// http://www.cs.princeton.edu/~rs/talks/LLRB/LLRB.pdf
// http://www.cs.princeton.edu/~rs/talks/LLRB/Java/RedBlackBST.java
// http://www.teachsolaisgames.com/articles/balanced_left_leaning.html
//
// Heavily modified by Miek Gieben for use in DNS zones.
package tree

import "github.com/miekg/dns"

const (
	td234 = iota
	bu23
)

// Operation mode of the LLRB tree.
const mode = bu23

func init() {
	if mode != td234 && mode != bu23 {
		panic("tree: unknown mode")
	}
}

// A Color represents the color of a Node.
type Color bool

const (
	// Red as false give us the defined behaviour that new nodes are red. Although this
	// is incorrect for the root node, that is resolved on the first insertion.
	red   Color = false
	black Color = true
)

// A Node represents a node in the LLRB tree.
type Node struct {
	Elem        *Elem
	Left, Right *Node
	Color       Color
}

// A Tree manages the root node of an LLRB tree. Public methods are exposed through this type.
type Tree struct {
	Root  *Node // Root node of the tree.
	Count int   // Number of elements stored.
}

// Helper methods

// color returns the effect color of a Node. A nil node returns black.
func (n *Node) color() Color {
	if n == nil {
		return black
	}
	return n.Color
}

// (a,c)b -rotL-> ((a,)b,)c
func (n *Node) rotateLeft() (root *Node) {
	// Assumes: n has two children.
	root = n.Right
	n.Right = root.Left
	root.Left = n
	root.Color = n.Color
	n.Color = red
	return
}

// (a,c)b -rotR-> (,(,c)b)a
func (n *Node) rotateRight() (root *Node) {
	// Assumes: n has two children.
	root = n.Left
	n.Left = root.Right
	root.Right = n
	root.Color = n.Color
	n.Color = red
	return
}

// (aR,cR)bB -flipC-> (aB,cB)bR | (aB,cB)bR -flipC-> (aR,cR)bB
func (n *Node) flipColors() {
	// Assumes: n has two children.
	n.Color = !n.Color
	n.Left.Color = !n.Left.Color
	n.Right.Color = !n.Right.Color
}

// fixUp ensures that black link balance is correct, that red nodes lean left,
// and that 4 nodes are split in the case of BU23 and properly balanced in TD234.
func (n *Node) fixUp() *Node {
	if n.Right.color() == red {
		if mode == td234 && n.Right.Left.color() == red {
			n.Right = n.Right.rotateRight()
		}
		n = n.rotateLeft()
	}
	if n.Left.color() == red && n.Left.Left.color() == red {
		n = n.rotateRight()
	}
	if mode == bu23 && n.Left.color() == red && n.Right.color() == red {
		n.flipColors()
	}
	return n
}

func (n *Node) moveRedLeft() *Node {
	n.flipColors()
	if n.Right.Left.color() == red {
		n.Right = n.Right.rotateRight()
		n = n.rotateLeft()
		n.flipColors()
		if mode == td234 && n.Right.Right.color() == red {
			n.Right = n.Right.rotateLeft()
		}
	}
	return n
}

func (n *Node) moveRedRight() *Node {
	n.flipColors()
	if n.Left.Left.color() == red {
		n = n.rotateRight()
		n.flipColors()
	}
	return n
}

// Len returns the number of elements stored in the Tree.
func (t *Tree) Len() int {
	return t.Count
}

// Search returns the first match of qname in the Tree.
func (t *Tree) Search(qname string) (*Elem, bool) {
	if t.Root == nil {
		return nil, false
	}
	n, res := t.Root.search(qname)
	if n == nil {
		return nil, res
	}
	return n.Elem, res
}

// search searches the tree for qname and type.
func (n *Node) search(qname string) (*Node, bool) {
	for n != nil {
		switch c := Less(n.Elem, qname); {
		case c == 0:
			return n, true
		case c < 0:
			n = n.Left
		default:
			n = n.Right
		}
	}

	return n, false
}

// Insert inserts rr into the Tree at the first match found
// with e or when a nil node is reached.
func (t *Tree) Insert(rr dns.RR) {
	var d int
	t.Root, d = t.Root.insert(rr)
	t.Count += d
	t.Root.Color = black
}

// insert inserts rr in to the tree.
func (n *Node) insert(rr dns.RR) (root *Node, d int) {
	if n == nil {
		return &Node{Elem: newElem(rr)}, 1
	} else if n.Elem == nil {
		n.Elem = newElem(rr)
		return n, 1
	}

	if mode == td234 {
		if n.Left.color() == red && n.Right.color() == red {
			n.flipColors()
		}
	}

	switch c := Less(n.Elem, rr.Header().Name); {
	case c == 0:
		n.Elem.Insert(rr)
	case c < 0:
		n.Left, d = n.Left.insert(rr)
	default:
		n.Right, d = n.Right.insert(rr)
	}

	if n.Right.color() == red && n.Left.color() == black {
		n = n.rotateLeft()
	}
	if n.Left.color() == red && n.Left.Left.color() == red {
		n = n.rotateRight()
	}

	if mode == bu23 {
		if n.Left.color() == red && n.Right.color() == red {
			n.flipColors()
		}
	}

	root = n

	return
}

// DeleteMin deletes the node with the minimum value in the tree.
func (t *Tree) DeleteMin() {
	if t.Root == nil {
		return
	}
	var d int
	t.Root, d = t.Root.deleteMin()
	t.Count += d
	if t.Root == nil {
		return
	}
	t.Root.Color = black
}

func (n *Node) deleteMin() (root *Node, d int) {
	if n.Left == nil {
		return nil, -1
	}
	if n.Left.color() == black && n.Left.Left.color() == black {
		n = n.moveRedLeft()
	}
	n.Left, d = n.Left.deleteMin()

	root = n.fixUp()

	return
}

// DeleteMax deletes the node with the maximum value in the tree.
func (t *Tree) DeleteMax() {
	if t.Root == nil {
		return
	}
	var d int
	t.Root, d = t.Root.deleteMax()
	t.Count += d
	if t.Root == nil {
		return
	}
	t.Root.Color = black
}

func (n *Node) deleteMax() (root *Node, d int) {
	if n.Left != nil && n.Left.color() == red {
		n = n.rotateRight()
	}
	if n.Right == nil {
		return nil, -1
	}
	if n.Right.color() == black && n.Right.Left.color() == black {
		n = n.moveRedRight()
	}
	n.Right, d = n.Right.deleteMax()

	root = n.fixUp()

	return
}

// Delete removes all RRs of type rr.Header().Rrtype from e. If after the deletion of rr the node is empty the
// entire node is deleted.
func (t *Tree) Delete(rr dns.RR) {
	if t.Root == nil {
		return
	}

	el, _ := t.Search(rr.Header().Name)
	if el == nil {
		return
	}
	el.Delete(rr)
	if el.Empty() {
		t.deleteNode(rr)
	}
}

// DeleteNode deletes the node that matches rr according to Less().
func (t *Tree) deleteNode(rr dns.RR) {
	if t.Root == nil {
		return
	}
	var d int
	t.Root, d = t.Root.delete(rr)
	t.Count += d
	if t.Root == nil {
		return
	}
	t.Root.Color = black
}

func (n *Node) delete(rr dns.RR) (root *Node, d int) {
	if Less(n.Elem, rr.Header().Name) < 0 {
		if n.Left != nil {
			if n.Left.color() == black && n.Left.Left.color() == black {
				n = n.moveRedLeft()
			}
			n.Left, d = n.Left.delete(rr)
		}
	} else {
		if n.Left.color() == red {
			n = n.rotateRight()
		}
		if n.Right == nil && Less(n.Elem, rr.Header().Name) == 0 {
			return nil, -1
		}
		if n.Right != nil {
			if n.Right.color() == black && n.Right.Left.color() == black {
				n = n.moveRedRight()
			}
			if Less(n.Elem, rr.Header().Name) == 0 {
				n.Elem = n.Right.min().Elem
				n.Right, d = n.Right.deleteMin()
			} else {
				n.Right, d = n.Right.delete(rr)
			}
		}
	}

	root = n.fixUp()
	return
}

// Min returns the minimum value stored in the tree.
func (t *Tree) Min() *Elem {
	if t.Root == nil {
		return nil
	}
	return t.Root.min().Elem
}

func (n *Node) min() *Node {
	for ; n.Left != nil; n = n.Left {
	}
	return n
}

// Max returns the maximum value stored in the tree.
func (t *Tree) Max() *Elem {
	if t.Root == nil {
		return nil
	}
	return t.Root.max().Elem
}

func (n *Node) max() *Node {
	for ; n.Right != nil; n = n.Right {
	}
	return n
}

// Prev returns the greatest value equal to or less than the qname according to Less().
func (t *Tree) Prev(qname string) (*Elem, bool) {
	if t.Root == nil {
		return nil, false
	}

	n := t.Root.floor(qname)
	if n == nil {
		return nil, false
	}
	return n.Elem, true
}

func (n *Node) floor(qname string) *Node {
	if n == nil {
		return nil
	}
	switch c := Less(n.Elem, qname); {
	case c == 0:
		return n
	case c <= 0:
		return n.Left.floor(qname)
	default:
		if r := n.Right.floor(qname); r != nil {
			return r
		}
	}
	return n
}

// Next returns the smallest value equal to or greater than the qname according to Less().
func (t *Tree) Next(qname string) (*Elem, bool) {
	if t.Root == nil {
		return nil, false
	}
	n := t.Root.ceil(qname)
	if n == nil {
		return nil, false
	}
	return n.Elem, true
}

func (n *Node) ceil(qname string) *Node {
	if n == nil {
		return nil
	}
	switch c := Less(n.Elem, qname); {
	case c == 0:
		return n
	case c > 0:
		return n.Right.ceil(qname)
	default:
		if l := n.Left.ceil(qname); l != nil {
			return l
		}
	}
	return n
}

/*
Copyright ©2012 The bíogo Authors. All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright
  notice, this list of conditions and the following disclaimer.
* Redistributions in binary form must reproduce the above copyright
  notice, this list of conditions and the following disclaimer in the
  documentation and/or other materials provided with the distribution.
* Neither the name of the bíogo project nor the names of its authors and
  contributors may be used to endorse or promote products derived from this
  software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/
