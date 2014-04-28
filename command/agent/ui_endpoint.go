package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"strings"
)

// UINodes is used to list the nodes in a given datacenter. We return a
// UINodeList which provides overview information for all the nodes
func (s *HTTPServer) UINodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Verify we have some DC, or use the default
	dc := strings.TrimPrefix(req.URL.Path, "/v1/internal/ui/nodes/")
	if dc == "" {
		dc = s.agent.config.Datacenter
	}

	// Try to ge ta node dump
	var dump structs.NodeDump
	if err := s.getNodeDump(resp, dc, &dump); err != nil {
		return nil, err
	}

	return dump, nil
}

// getNodeDump is used to get a dump of all node data. We make a best effort by
// reading stale data in the case of an availability outage.
func (s *HTTPServer) getNodeDump(resp http.ResponseWriter, dc string, dump *structs.NodeDump) error {
	args := structs.DCSpecificRequest{Datacenter: dc}
	var out structs.IndexedNodeDump
	defer setMeta(resp, &out.QueryMeta)

START:
	if err := s.agent.RPC("Internal.NodeDump", &args, &out); err != nil {
		// Retry the request allowing stale data if no leader. The UI should continue
		// to function even during an outage
		if strings.Contains(err.Error(), structs.ErrNoLeader.Error()) && !args.AllowStale {
			args.AllowStale = true
			goto START
		}
		return err
	}

	// Set the result
	*dump = out.Dump
	return nil
}
