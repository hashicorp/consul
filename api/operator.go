package api

import (
	"github.com/hashicorp/raft"
)

// Operator can be used to perform low-level operator tasks for Consul.
type Operator struct {
	c *Client
}

// Operator returns a handle to the operator endpoints.
func (c *Client) Operator() *Operator {
	return &Operator{c}
}

// RaftConfigration is returned when querying for the current Raft configuration.
// This has the low-level Raft structure, as well as some supplemental
// information from Consul.
type RaftConfiguration struct {
	// Configuration is the low-level Raft configuration structure.
	Configuration raft.Configuration

	// NodeMap maps IDs in the Raft configuration to node names known by
	// Consul. It's possible that not all configuration entries may have
	// an entry here if the node isn't known to Consul. Given how this is
	// generated, this may also contain entries that aren't present in the
	// Raft configuration.
	NodeMap map[raft.ServerID]string

	// Leader is the ID of the current Raft leader. This may be blank if
	// there isn't one.
	Leader raft.ServerID
}

// RaftGetConfiguration is used to query the current Raft peer set.
func (op *Operator) RaftGetConfiguration(q *QueryOptions) (*RaftConfiguration, error) {
	r := op.c.newRequest("GET", "/v1/operator/raft/configuration")
	r.setQueryOptions(q)
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out RaftConfiguration
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RaftRemovePeerByAddress is used to kick a stale peer (one that it in the Raft
// quorum but no longer known to Serf or the catalog) by address in the form of
// "IP:port".
func (op *Operator) RaftRemovePeerByAddress(address raft.ServerAddress, q *WriteOptions) error {
	r := op.c.newRequest("DELETE", "/v1/operator/raft/peer")
	r.setWriteOptions(q)

	// TODO (slackpad) Currently we made address a query parameter. Once
	// IDs are in place this will be DELETE /v1/operator/raft/peer/<id>.
	r.params.Set("address", string(address))

	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return err
	}

	resp.Body.Close()
	return nil
}
