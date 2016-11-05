package tree

import "fmt"

// Print prints a Tree. Main use is to aid in debugging.
func (t *Tree) Print() {
	if t.Root == nil {
		fmt.Println("<nil>")
	}
	t.Root.print()
}

func (n *Node) print() {
	q := NewQueue()
	q.Push(n)

	nodesInCurrentLevel := 1
	nodesInNextLevel := 0

	for !q.Empty() {
		do := q.Pop()
		nodesInCurrentLevel--

		if do != nil {
			fmt.Print(do.Elem.Name(), " ")
			q.Push(do.Left)
			q.Push(do.Right)
			nodesInNextLevel += 2
		}
		if nodesInCurrentLevel == 0 {
			fmt.Println()
		}
		nodesInCurrentLevel = nodesInNextLevel
		nodesInNextLevel = 0
	}
	fmt.Println()
}

type queue []*Node

func NewQueue() queue {
	q := queue([]*Node{})
	return q
}

func (q *queue) Push(n *Node) {
	*q = append(*q, n)
}

func (q *queue) Pop() *Node {
	n := (*q)[0]
	*q = (*q)[1:]
	return n
}

func (q *queue) Empty() bool {
	return len(*q) == 0
}
