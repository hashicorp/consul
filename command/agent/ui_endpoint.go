package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"sort"
	"strings"
)

// ServiceSummary is used to summarize a service
type ServiceSummary struct {
	Name           string
	Nodes          []string
	ChecksPassing  int
	ChecksWarning  int
	ChecksCritical int
}

// UINodes is used to list the nodes in a given datacenter. We return a
// NodeDump which provides overview information for all the nodes
func (s *HTTPServer) UINodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the datacenter
	var dc string
	s.parseDC(req, &dc)

	// Try to ge ta node dump
	var dump structs.NodeDump
	if err := s.getNodeDump(resp, dc, "", &dump); err != nil {
		return nil, err
	}

	return dump, nil
}

// UINodeInfo is used to get info on a single node in a given datacenter. We return a
// NodeInfo which provides overview information for the node
func (s *HTTPServer) UINodeInfo(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the datacenter
	var dc string
	s.parseDC(req, &dc)

	// Verify we have some DC, or use the default
	node := strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/node/")
	if node == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing node name"))
		return nil, nil
	}

	// Try to get a node dump
	var dump structs.NodeDump
	if err := s.getNodeDump(resp, dc, node, &dump); err != nil {
		return nil, err
	}

	// Return only the first entry
	if len(dump) > 0 {
		return dump[0], nil
	}
	return nil, nil
}

// getNodeDump is used to get a dump of all node data. We make a best effort by
// reading stale data in the case of an availability outage.
func (s *HTTPServer) getNodeDump(resp http.ResponseWriter, dc, node string, dump *structs.NodeDump) error {
	var args interface{}
	var method string
	var allowStale *bool

	if node == "" {
		raw := structs.DCSpecificRequest{Datacenter: dc}
		method = "Internal.NodeDump"
		allowStale = &raw.AllowStale
		args = &raw
	} else {
		raw := &structs.NodeSpecificRequest{Datacenter: dc, Node: node}
		method = "Internal.NodeInfo"
		allowStale = &raw.AllowStale
		args = &raw
	}
	var out structs.IndexedNodeDump
	defer setMeta(resp, &out.QueryMeta)

START:
	if err := s.agent.RPC(method, args, &out); err != nil {
		// Retry the request allowing stale data if no leader. The UI should continue
		// to function even during an outage
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !*allowStale {
			*allowStale = true
			goto START
		}
		return err
	}

	// Set the result
	*dump = out.Dump
	return nil
}

// UIServices is used to list the services in a given datacenter. We return a
// ServiceSummary which provides overview information for the service
func (s *HTTPServer) UIServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Get the datacenter
	var dc string
	s.parseDC(req, &dc)

	// Get the full node dump...
	var dump structs.NodeDump
	if err := s.getNodeDump(resp, dc, "", &dump); err != nil {
		return nil, err
	}

	// Generate the summary
	return summarizeServices(dump), nil
}

func summarizeServices(dump structs.NodeDump) []*ServiceSummary {
	// Collect the summary information
	var services []string
	summary := make(map[string]*ServiceSummary)
	getService := func(service string) *ServiceSummary {
		serv, ok := summary[service]
		if !ok {
			serv = &ServiceSummary{Name: service}
			summary[service] = serv
			services = append(services, service)
		}
		return serv
	}

	// Aggregate all the node information
	for _, node := range dump {
		nodeServices := make([]*ServiceSummary, len(node.Services))
		for idx, service := range node.Services {
			sum := getService(service.Service)
			sum.Nodes = append(sum.Nodes, node.Node)
			nodeServices[idx] = sum
		}
		for _, check := range node.Checks {
			var services []*ServiceSummary
			if check.ServiceName == "" {
				services = nodeServices
			} else {
				services = []*ServiceSummary{getService(check.ServiceName)}
			}
			for _, sum := range services {
				switch check.Status {
				case structs.HealthPassing:
					sum.ChecksPassing++
				case structs.HealthWarning:
					sum.ChecksWarning++
				case structs.HealthCritical:
					sum.ChecksCritical++
				}
			}
		}
	}

	// Return the services in sorted order
	sort.Strings(services)
	output := make([]*ServiceSummary, len(summary))
	for idx, service := range services {
		// Sort the nodes
		sum := summary[service]
		sort.Strings(sum.Nodes)
		output[idx] = sum
	}
	return output
}
