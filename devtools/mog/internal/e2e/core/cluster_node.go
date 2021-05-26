package core

import "github.com/hashicorp/mog/internal/e2e/core/inner"

type Label string

type ClusterNode struct {
	ID string
	// Labels []Label
	// WorkPointer []*Workload

	O *Other
	I inner.Inner

	F1 Workload  // for testing struct-to-struct
	F2 *Workload // for testing ptr-to-ptr
	F3 Workload  // for testing ptr-to-struct
	F4 *Workload // for testing struct-to-ptr

	S1 []string  // for testing struct-to-struct for basic slices
	S2 []*string // for testing ptr-to-ptr for basic slices
	S3 []string  // for testing ptr-to-struct for basic slices
	S4 []*string // for testing struct-to-ptr for basic slices

	S5 []Workload  // for testing struct-to-struct for struct slices
	S6 []*Workload // for testing ptr-to-ptr for struct slices
	S7 []Workload  // for testing ptr-to-struct for struct slices
	S8 []*Workload // for testing struct-to-ptr for struct slices

	S9  StringSlice
	S10 []string

	S11 WorkloadSlice
	S12 []*Workload
	S13 WorkloadSlice

	M1 map[string]string  // for testing struct-to-string for basic map values
	M2 map[string]*string // for testing ptr-to-ptr for basic map values
	M3 map[string]string  // for testing ptr-to-string for basic map values
	M4 map[string]*string // for testing struct-to-ptr for basic map values

	M5 map[string]Workload
	M6 map[string]*Workload
	M7 map[string]Workload
	M8 map[string]*Workload

	// S1 Workload  // for testing struct-to-struct for slices
	// S2 *Workload // for testing ptr-to-ptr for slices
	// S3 Workload  // for testing ptr-to-struct for slices
	// S4 *Workload // for testing struct-to-ptr for slices
}

type StringSlice []string
type WorkloadSlice []*Workload

type Workload struct {
	ID string
}

type Other struct {
	N int
}
