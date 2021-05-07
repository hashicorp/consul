package core

type Label string

type ClusterNode struct {
	ID string
	// Labels []Label
	// WorkPointer []*Workload

	F1 Workload  // for testing struct-to-struct
	F2 *Workload // for testing ptr-to-ptr
	F3 Workload  // for testing ptr-to-struct
	F4 *Workload // for testing struct-to-ptr
}

type Workload struct {
	ID string
}
