package core

type Label string

type ClusterNode struct {
	ID     string
	Labels []Label
}
