package agent

import (
	"net/http"

	"github.com/hashicorp/consul/consul/structs"
)

// Snapshot handles requests to take and restore snapshots. This uses a special
// mechanism to make the RPC since we potentially stream large amounts of data
// as part of these requests.
func (s *HTTPServer) Snapshot(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.SnapshotRequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	switch req.Method {
	case "GET":
		args.Op = structs.SnapshotSave

	case "PUT":
		args.Op = structs.SnapshotRestore

	default:
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	if err := s.agent.SnapshotRPC(&args, req.Body, resp); err != nil {
		return nil, err
	}
	return nil, nil
}
