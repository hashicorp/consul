// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"bytes"
	"fmt"
	"text/tabwriter"
)

// ComputeRelationships will analyze a full topology and generate all of the
// caller/upstream information for all of them.
func (t *Topology) ComputeRelationships() []Relationship {
	var out []Relationship
	for _, cluster := range t.Clusters {
		for _, n := range cluster.Nodes {
			for _, w := range n.Workloads {
				for _, us := range w.Upstreams {
					out = append(out, Relationship{
						Caller:   w,
						Upstream: us,
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
	fmt.Fprintf(w, "CALLER\tnode\tservice\tport\tDEST\tservice\t\n")
	for _, r := range ships {
		fmt.Fprintf(w,
			"%s\t%s\t%s\t%d\t%s\t%s\t\n",
			r.callingCluster(),
			r.Caller.Node.ID().String(),
			r.Caller.ID.String(),
			r.Upstream.LocalPort,
			r.upstreamCluster(),
			r.Upstream.ID.String(),
		)
	}
	fmt.Fprintf(w, "\t\t\t\t\t\t\n")

	w.Flush()
	return buf.String()
}

type Relationship struct {
	Caller   *Workload
	Upstream *Upstream
}

func (r Relationship) String() string {
	return fmt.Sprintf(
		"%s on %s in %s via :%d => %s in %s",
		r.Caller.ID.String(),
		r.Caller.Node.ID().String(),
		r.callingCluster(),
		r.Upstream.LocalPort,
		r.Upstream.ID.String(),
		r.upstreamCluster(),
	)
}

func (r Relationship) callingCluster() string {
	return r.Caller.Node.Cluster
}

func (r Relationship) upstreamCluster() string {
	return r.Upstream.Cluster
}
