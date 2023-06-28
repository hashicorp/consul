package peering

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/hashicorp/consul-topology/topology"
)

// ComputeRelationships will analyze a full topology and generate all of the
// downstream/upstream information for all of them.
func ComputeRelationships(topo *topology.Topology) []Relationship {
	var out []Relationship
	for _, cluster := range topo.Clusters {
		for _, n := range cluster.Nodes {
			for _, s := range n.Services {
				for _, u := range s.Upstreams {
					out = append(out, Relationship{
						Caller:   s,
						Upstream: u,
					})
				}
			}
		}
	}
	return out
}

// RenderRelationships will take the output of ComputeRelationships and display
// it in tabular form.
func RenderRelationships(ships []Relationship) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', tabwriter.Debug)
	fmt.Fprintf(w, "DOWN\tnode\tservice\tport\tUP\tservice\t\n")
	for _, r := range ships {
		fmt.Fprintf(w,
			"%s\t%s\t%s\t%d\t%s\t%s\t\n",
			r.downCluster(),
			r.Caller.Node.ID().String(),
			r.Caller.ID.String(),
			r.Upstream.LocalPort,
			r.upCluster(),
			r.Upstream.ID.String(),
		)
	}
	fmt.Fprintf(w, "\t\t\t\t\t\t\n")

	w.Flush()
	return buf.String()
}

type Relationship struct {
	Caller   *topology.Service
	Upstream *topology.Upstream
}

func (r Relationship) String() string {
	return fmt.Sprintf(
		"%s on %s in %s via :%d => %s in %s",
		r.Caller.ID.String(),
		r.Caller.Node.ID().String(),
		r.downCluster(),
		r.Upstream.LocalPort,
		r.Upstream.ID.String(),
		r.upCluster(),
	)
}

func (r Relationship) downCluster() string {
	return r.Caller.Node.Cluster
}

func (r Relationship) upCluster() string {
	return r.Upstream.Cluster
}
