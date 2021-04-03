package sourcepkg

// Node source structure for e2e testing mog.
//
// mog annotation:
//
// name=Core
// target=github.com/hashicorp/mog/internal/e2e/core.ClusterNode
// output=node_gen.go
type Node struct {
	ID     string
	Weight int64
	Labels []string
	Meta   map[string]interface{}
	Work   []Workload
}

type Workload struct {
	ID string
}
