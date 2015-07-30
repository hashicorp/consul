package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
)

// coordinateDisabled handles all the endpoints when coordinates are not enabled,
// returning an error message.
func coordinateDisabled(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	resp.WriteHeader(401)
	resp.Write([]byte("Coordinate support disabled"))
	return nil, nil
}

// CoordinateDatacenters returns the WAN nodes in each datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var out []structs.DatacenterMap
	if err := s.agent.RPC("Coordinate.ListDatacenters", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CoordinateNodes returns the LAN nodes in the given datacenter, along with
// raw network coordinates.
func (s *HTTPServer) CoordinateNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedCoordinates
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Coordinate.ListNodes", &args, &out); err != nil {
		return nil, err
	}
	return out.Coordinates, nil
}
