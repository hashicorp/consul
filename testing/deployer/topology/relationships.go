// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package topology

import (
	"bytes"
	"fmt"
	"text/tabwriter"
)

// ComputeRelationships will analyze a full topology and generate all of the
// caller/destination information for all of them.
func (t *Topology) ComputeRelationships() []Relationship {
	var out []Relationship
	for _, cluster := range t.Clusters {
		for _, n := range cluster.Nodes {
			for _, w := range n.Workloads {
				for _, dest := range w.Upstreams {
					out = append(out, Relationship{
						Caller:      w,
						Destination: dest,
						Upstream:    dest,
					})
				}
				for _, dest := range w.ImpliedUpstreams {
					out = append(out, Relationship{
						Caller:      w,
						Destination: dest,
						Upstream:    dest,
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
		suffix := ""
		if r.Destination.Implied {
			suffix = " (implied)"
		}
		fmt.Fprintf(w,
			"%s\t%s\t%s\t%d\t%s\t%s\t\n",
			r.callingCluster(),
			r.Caller.Node.ID().String(),
			r.Caller.ID.String(),
			r.Destination.LocalPort,
			r.destinationCluster(),
			r.Destination.ID.String()+suffix,
		)
	}
	fmt.Fprintf(w, "\t\t\t\t\t\t\n")

	w.Flush()
	return buf.String()
}

type Relationship struct {
	Caller      *Workload
	Destination *Destination

	// Deprecated: Destination
	Upstream *Destination
}

func (r Relationship) String() string {
	suffix := ""
	if r.Destination.PortName != "" {
		suffix = " port " + r.Destination.PortName
	}
	return fmt.Sprintf(
		"%s on %s in %s via :%d => %s in %s%s",
		r.Caller.ID.String(),
		r.Caller.Node.ID().String(),
		r.callingCluster(),
		r.Destination.LocalPort,
		r.Destination.ID.String(),
		r.destinationCluster(),
		suffix,
	)
}

func (r Relationship) callingCluster() string {
	return r.Caller.Node.Cluster
}

func (r Relationship) destinationCluster() string {
	return r.Destination.Cluster
}
