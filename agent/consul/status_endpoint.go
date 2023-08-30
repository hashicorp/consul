package consul

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/consul/agent/structs"
)

// Status endpoint is used to check on server status
type Status struct {
	server *Server
}

// Ping is used to just check for connectivity
func (s *Status) Ping(args EmptyReadRequest, reply *struct{}) error {
	return nil
}

// Leader is used to get the address of the leader
func (s *Status) Leader(args *structs.DCSpecificRequest, reply *string) error {
	// not using the regular forward function as it does a bunch of stuff we
	// dont want like verifying consistency etc. We just want to enable DC
	// forwarding
	if args.Datacenter != "" && args.Datacenter != s.server.config.Datacenter {
		return s.server.forwardDC("Status.Leader", args.Datacenter, args, reply)
	}

	leader := string(s.server.raft.Leader())
	if leader != "" {
		*reply = leader
	} else {
		*reply = ""
	}
	return nil
}

// Peers is used to get all the Raft peers
func (s *Status) Peers(args *structs.DCSpecificRequest, reply *[]string) error {
	// not using the regular forward function as it does a bunch of stuff we
	// dont want like verifying consistency etc. We just want to enable DC
	// forwarding
	if args.Datacenter != "" && args.Datacenter != s.server.config.Datacenter {
		return s.server.forwardDC("Status.Peers", args.Datacenter, args, reply)
	}

	future := s.server.raft.GetConfiguration()
	if err := future.Error(); err != nil {
		return err
	}

	for _, server := range future.Configuration().Servers {
		*reply = append(*reply, string(server.Address))
	}
	return nil
}

// EmptyReadRequest implements the interface used by middleware.RequestRecorder
// to communicate properties of requests.
type EmptyReadRequest struct{}

func (e EmptyReadRequest) IsRead() bool {
	return true
}

// RaftStats is used by Autopilot to query the raft stats of the local server.
func (s *Status) RaftStats(args EmptyReadRequest, reply *structs.RaftStats) error {
	stats := s.server.raft.Stats()

	var err error
	reply.LastContact = stats["last_contact"]
	reply.LastIndex, err = strconv.ParseUint(stats["last_log_index"], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing server's last_log_index value: %w", err)
	}
	reply.LastTerm, err = strconv.ParseUint(stats["last_log_term"], 10, 64)
	if err != nil {
		return fmt.Errorf("error parsing server's last_log_term value: %w", err)
	}

	return nil
}
