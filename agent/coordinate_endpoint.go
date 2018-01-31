package agent

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// checkCoordinateDisabled will return a standard response if coordinates are
// disabled. This returns true if they are disabled and we should not continue.
func (s *HTTPServer) checkCoordinateDisabled(resp http.ResponseWriter, req *http.Request) bool {
	if !s.agent.config.DisableCoordinates {
		return false
	}

	resp.WriteHeader(http.StatusUnauthorized)
	fmt.Fprint(resp, "Coordinate support disabled")
	return true
}

// sorter wraps a coordinate list and implements the sort.Interface to sort by
// node name.
type sorter struct {
	coordinates structs.Coordinates
}

// See sort.Interface.
func (s *sorter) Len() int {
	return len(s.coordinates)
}

// See sort.Interface.
func (s *sorter) Swap(i, j int) {
	s.coordinates[i], s.coordinates[j] = s.coordinates[j], s.coordinates[i]
}

// See sort.Interface.
func (s *sorter) Less(i, j int) bool {
	return s.coordinates[i].Node < s.coordinates[j].Node
}

// CoordinateDatacenters returns the WAN nodes in each datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkCoordinateDisabled(resp, req) {
		return nil, nil
	}
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	var out []structs.DatacenterMap
	if err := s.agent.RPC("Coordinate.ListDatacenters", struct{}{}, &out); err != nil {
		for i := range out {
			sort.Sort(&sorter{out[i].Coordinates})
		}
		return nil, err
	}

	// Use empty list instead of nil (these aren't really possible because
	// Serf will give back a default coordinate and there's always one DC,
	// but it's better to be explicit about what we want here).
	for i := range out {
		if out[i].Coordinates == nil {
			out[i].Coordinates = make(structs.Coordinates, 0)
		}
	}
	if out == nil {
		out = make([]structs.DatacenterMap, 0)
	}
	return out, nil
}

// CoordinateNodes returns the LAN nodes in the given datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkCoordinateDisabled(resp, req) {
		return nil, nil
	}
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedCoordinates
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Coordinate.ListNodes", &args, &out); err != nil {
		sort.Sort(&sorter{out.Coordinates})
		return nil, err
	}

	return filterCoordinates(req, out.Coordinates), nil
}

// CoordinateNode returns the LAN node in the given datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateNode(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkCoordinateDisabled(resp, req) {
		return nil, nil
	}
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	node := strings.TrimPrefix(req.URL.Path, "/v1/coordinate/node/")
	args := structs.NodeSpecificRequest{Node: node}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedCoordinates
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Coordinate.Node", &args, &out); err != nil {
		return nil, err
	}

	result := filterCoordinates(req, out.Coordinates)
	if len(result) == 0 {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	return result, nil
}

func filterCoordinates(req *http.Request, in structs.Coordinates) structs.Coordinates {
	out := structs.Coordinates{}

	if in == nil {
		return out
	}

	segment := ""
	v, filterBySegment := req.URL.Query()["segment"]
	if filterBySegment && len(v) > 0 {
		segment = v[0]
	}

	for _, c := range in {
		if filterBySegment && c.Segment != segment {
			continue
		}
		out = append(out, c)
	}
	return out
}

// CoordinateUpdate inserts or updates the LAN coordinate of a node.
func (s *HTTPServer) CoordinateUpdate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if s.checkCoordinateDisabled(resp, req) {
		return nil, nil
	}
	if req.Method != "PUT" {
		return nil, MethodNotAllowedError{req.Method, []string{"PUT"}}
	}

	args := structs.CoordinateUpdateRequest{}
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}
	s.parseDC(req, &args.Datacenter)

	var reply struct{}
	if err := s.agent.RPC("Coordinate.Update", &args, &reply); err != nil {
		return nil, err
	}

	return nil, nil
}
