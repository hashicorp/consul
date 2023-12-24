// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package sprawl

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/hashicorp/consul/api"
)

// PrintDetails will dump relevant addressing and naming data to the logger for
// human interaction purposes.
func (s *Sprawl) PrintDetails() error {
	det := logDetails{
		TopologyID: s.topology.ID,
	}

	for _, cluster := range s.topology.Clusters {
		client := s.clients[cluster.Name]

		var cfg *api.RaftConfiguration
		var err error
		err = retry.Do(
			func() error {
				cfg, err = client.Operator().RaftGetConfiguration(nil)
				if err != nil {
					return fmt.Errorf("error get raft config: %w", err)
				}
				return nil
			},
			retry.MaxDelay(5*time.Second),
			retry.Attempts(15),
		)
		if err != nil {
			return fmt.Errorf("could not get raft config for cluster %q: %w", cluster.Name, err)
		}

		var leaderNode string
		for _, svr := range cfg.Servers {
			if svr.Leader {
				leaderNode = strings.TrimSuffix(svr.Node, "-pod")
			}
		}

		cd := clusterDetails{
			Name:   cluster.Name,
			Leader: leaderNode,
		}

		for _, node := range cluster.Nodes {
			if node.Disabled {
				continue
			}

			var addrs []string
			for _, addr := range node.Addresses {
				addrs = append(addrs, addr.Network+"="+addr.IPAddress)
			}
			sort.Strings(addrs)

			if node.IsServer() {
				cd.Apps = append(cd.Apps, appDetail{
					Type:        "server",
					Container:   node.DockerName(),
					Addresses:   addrs,
					ExposedPort: node.ExposedPort(8500),
				})
			}

			for _, wrk := range node.Workloads {
				if wrk.IsMeshGateway {
					cd.Apps = append(cd.Apps, appDetail{
						Type:                  "mesh-gateway",
						Container:             node.DockerName(),
						ExposedPort:           node.ExposedPort(wrk.Port),
						ExposedEnvoyAdminPort: node.ExposedPort(wrk.EnvoyAdminPort),
						Addresses:             addrs,
						Service:               wrk.ID.String(),
					})
				} else {
					ports := make(map[string]int)
					for name, port := range wrk.Ports {
						ports[name] = node.ExposedPort(port.Number)
					}
					cd.Apps = append(cd.Apps, appDetail{
						Type:                  "app",
						Container:             node.DockerName(),
						ExposedPort:           node.ExposedPort(wrk.Port),
						ExposedPorts:          ports,
						ExposedEnvoyAdminPort: node.ExposedPort(wrk.EnvoyAdminPort),
						Addresses:             addrs,
						Service:               wrk.ID.String(),
					})
				}
			}
		}

		det.Clusters = append(det.Clusters, cd)
	}

	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 0, 3, ' ', tabwriter.Debug)

	score := map[string]int{
		"server":       0,
		"mesh-gateway": 1,
		"app":          2,
	}

	for _, cluster := range det.Clusters {
		fmt.Fprintf(w, "CLUSTER\tTYPE\tCONTAINER\tNAME\tADDRS\tPORTS\t\n")
		sort.Slice(cluster.Apps, func(i, j int) bool {
			a := cluster.Apps[i]
			b := cluster.Apps[j]

			asc := score[a.Type]
			bsc := score[b.Type]

			if asc < bsc {
				return true
			} else if asc > bsc {
				return false
			}

			if a.Container < b.Container {
				return true
			} else if a.Container > b.Container {
				return false
			}

			return a.Service < b.Service
		})
		for _, d := range cluster.Apps {
			if d.Type == "server" && d.Container == cluster.Leader {
				d.Type = "leader"
			}
			var portStr string
			if len(d.ExposedPorts) > 0 {
				var out []string
				for name, exposed := range d.ExposedPorts {
					out = append(out, fmt.Sprintf("app:%s=%d", name, exposed))
				}
				sort.Strings(out)
				portStr = strings.Join(out, " ")
			} else {
				portStr = "app=" + strconv.Itoa(d.ExposedPort)
			}
			if d.ExposedEnvoyAdminPort > 0 {
				portStr += " envoy=" + strconv.Itoa(d.ExposedEnvoyAdminPort)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t\n",
				cluster.Name,
				d.Type,
				d.Container,
				d.Service,
				strings.Join(d.Addresses, ", "),
				portStr,
			)
		}
		fmt.Fprintf(w, "\t\t\t\t\t\n")
	}

	w.Flush()

	s.logger.Debug("CURRENT SPRAWL DETAILS", "details", buf.String())

	return nil
}

type logDetails struct {
	TopologyID string
	Clusters   []clusterDetails
}

type clusterDetails struct {
	Name string

	Leader string
	Apps   []appDetail
}

type appDetail struct {
	Type                  string // server|mesh-gateway|app
	Container             string
	Addresses             []string
	ExposedPort           int            `json:",omitempty"`
	ExposedPorts          map[string]int `json:",omitempty"`
	ExposedEnvoyAdminPort int            `json:",omitempty"`
	// just services
	Service string `json:",omitempty"`
}
