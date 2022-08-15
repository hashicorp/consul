package agent

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// checkCoordinateDisabled will return an unauthorized error if coordinates are
// disabled. Otherwise, a nil error will be returned.
func (s *HTTPHandlers) checkCoordinateDisabled() error {
	if !s.agent.config.DisableCoordinates {
		return nil
	}
	return HTTPError{StatusCode: http.StatusUnauthorized, Reason: "Coordinate support disabled"}
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
func (s *HTTPHandlers) CoordinateDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if err := s.checkCoordinateDisabled(); err != nil {
		return nil, err
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
func (s *HTTPHandlers) CoordinateNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if err := s.checkCoordinateDisabled(); err != nil {
		return nil, err
	}

	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMetaPartition(req, &args.EnterpriseMeta); err != nil {
		return nil, err
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
func (s *HTTPHandlers) CoordinateNode(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if err := s.checkCoordinateDisabled(); err != nil {
		return nil, err
	}

	node := strings.TrimPrefix(req.URL.Path, "/v1/coordinate/node/")
	args := structs.NodeSpecificRequest{Node: node}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMetaPartition(req, &args.EnterpriseMeta); err != nil {
		return nil, err
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
func (s *HTTPHandlers) CoordinateUpdate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if err := s.checkCoordinateDisabled(); err != nil {
		return nil, err
	}

	args := structs.CoordinateUpdateRequest{}
	if err := decodeBody(req.Body, &args); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	var reply struct{}
	if err := s.agent.RPC("Coordinate.Update", &args, &reply); err != nil {
		return nil, err
	}

	return nil, nil
}
