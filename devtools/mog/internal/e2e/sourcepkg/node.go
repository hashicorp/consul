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
	// Labels []string
	Meta map[string]interface{}
	Work []Workload
	// WorkPointer []*Workload

	F1 Workload  // for testing struct-to-struct
	F2 *Workload // for testing ptr-to-ptr
	F3 *Workload // for testing ptr-to-struct
	F4 Workload  // for testing struct-to-ptr
}

// mog annotation:
//
// name=Core
// target=github.com/hashicorp/mog/internal/e2e/core.Workload
// output=node_gen.go
type Workload struct {
	ID string
}
