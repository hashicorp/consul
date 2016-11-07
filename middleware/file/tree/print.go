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
	q := newQueue()
	q.push(n)

	nodesInCurrentLevel := 1
	nodesInNextLevel := 0

	for !q.empty() {
		do := q.pop()
		nodesInCurrentLevel--

		if do != nil {
			fmt.Print(do.Elem.Name(), " ")
			q.push(do.Left)
			q.push(do.Right)
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

// newQueue returns a new queue.
func newQueue() queue {
	q := queue([]*Node{})
	return q
}

// push pushes n to the end of the queue.
func (q *queue) push(n *Node) {
	*q = append(*q, n)
}

// pop pops the first element off the queue.
func (q *queue) pop() *Node {
	n := (*q)[0]
	*q = (*q)[1:]
	return n
}

// empty returns true when the queue containes zero nodes.
func (q *queue) empty() bool {
	return len(*q) == 0
}
