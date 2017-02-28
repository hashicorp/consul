package api

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Operator can be used to perform low-level operator tasks for Consul.
type Operator struct {
	c *Client
}

// Operator returns a handle to the operator endpoints.
func (c *Client) Operator() *Operator {
	return &Operator{c}
}

// RaftServer has information about a server in the Raft configuration.
type RaftServer struct {
	// ID is the unique ID for the server. These are currently the same
	// as the address, but they will be changed to a real GUID in a future
	// release of Consul.
	ID string

	// Node is the node name of the server, as known by Consul, or this
	// will be set to "(unknown)" otherwise.
	Node string

	// Address is the IP:port of the server, used for Raft communications.
	Address string

	// Leader is true if this server is the current cluster leader.
	Leader bool

	// Voter is true if this server has a vote in the cluster. This might
	// be false if the server is staging and still coming online, or if
	// it's a non-voting server, which will be added in a future release of
	// Consul.
	Voter bool
}

// RaftConfigration is returned when querying for the current Raft configuration.
type RaftConfiguration struct {
	// Servers has the list of servers in the Raft configuration.
	Servers []*RaftServer

	// Index has the Raft index of this configuration.
	Index uint64
}

// keyringRequest is used for performing Keyring operations
type keyringRequest struct {
	Key string
}

// KeyringResponse is returned when listing the gossip encryption keys
type KeyringResponse struct {
	// Whether this response is for a WAN ring
	WAN bool

	// The datacenter name this request corresponds to
	Datacenter string

	// A map of the encryption keys to the number of nodes they're installed on
	Keys map[string]int

	// The total number of nodes in this ring
	NumNodes int
}

// AutopilotConfiguration is used for querying/setting the Autopilot configuration.
// Autopilot helps manage operator tasks related to Consul servers like removing
// failed servers from the Raft quorum.
type AutopilotConfiguration struct {
	// CleanupDeadServers controls whether to remove dead servers from the Raft
	// peer list when a new server joins
	CleanupDeadServers bool

	// CreateIndex holds the index corresponding the creation of this configuration.
	// This is a read-only field.
	CreateIndex uint64

	// ModifyIndex will be set to the index of the last update when retrieving the
	// Autopilot configuration. Resubmitting a configuration with
	// AutopilotCASConfiguration will perform a check-and-set operation which ensures
	// there hasn't been a subsequent update since the configuration was retrieved.
	ModifyIndex uint64
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
func (op *Operator) RaftRemovePeerByAddress(address string, q *WriteOptions) error {
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

// KeyringInstall is used to install a new gossip encryption key into the cluster
func (op *Operator) KeyringInstall(key string, q *WriteOptions) error {
	r := op.c.newRequest("POST", "/v1/operator/keyring")
	r.setWriteOptions(q)
	r.obj = keyringRequest{
		Key: key,
	}
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// KeyringList is used to list the gossip keys installed in the cluster
func (op *Operator) KeyringList(q *QueryOptions) ([]*KeyringResponse, error) {
	r := op.c.newRequest("GET", "/v1/operator/keyring")
	r.setQueryOptions(q)
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out []*KeyringResponse
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// KeyringRemove is used to remove a gossip encryption key from the cluster
func (op *Operator) KeyringRemove(key string, q *WriteOptions) error {
	r := op.c.newRequest("DELETE", "/v1/operator/keyring")
	r.setWriteOptions(q)
	r.obj = keyringRequest{
		Key: key,
	}
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// KeyringUse is used to change the active gossip encryption key
func (op *Operator) KeyringUse(key string, q *WriteOptions) error {
	r := op.c.newRequest("PUT", "/v1/operator/keyring")
	r.setWriteOptions(q)
	r.obj = keyringRequest{
		Key: key,
	}
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// AutopilotGetConfiguration is used to query the current Autopilot configuration.
func (op *Operator) AutopilotGetConfiguration(q *QueryOptions) (*AutopilotConfiguration, error) {
	r := op.c.newRequest("GET", "/v1/operator/autopilot/configuration")
	r.setQueryOptions(q)
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out AutopilotConfiguration
	if err := decodeBody(resp, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AutopilotSetConfiguration is used to set the current Autopilot configuration.
func (op *Operator) AutopilotSetConfiguration(conf *AutopilotConfiguration, q *WriteOptions) error {
	r := op.c.newRequest("PUT", "/v1/operator/autopilot/configuration")
	r.setWriteOptions(q)
	r.obj = conf
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// AutopilotCASConfiguration is used to perform a Check-And-Set update on the
// Autopilot configuration. The ModifyIndex value will be respected. Returns
// true on success or false on failures.
func (op *Operator) AutopilotCASConfiguration(conf *AutopilotConfiguration, q *WriteOptions) (bool, error) {
	r := op.c.newRequest("PUT", "/v1/operator/autopilot/configuration")
	r.setWriteOptions(q)
	r.params.Set("cas", strconv.FormatUint(conf.ModifyIndex, 10))
	r.obj = conf
	_, resp, err := requireOK(op.c.doRequest(r))
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return false, fmt.Errorf("Failed to read response: %v", err)
	}
	res := strings.Contains(string(buf.Bytes()), "true")

	return res, nil
}
