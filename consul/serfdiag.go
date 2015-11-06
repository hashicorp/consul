package consul

import (
	"fmt"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/serf/serf"
	"log"
	"os"
	"strings"
	"time"
)

// NodeResponse is used to return the response of a serf query
type NodeResponse struct {
	From    string
	Payload []byte
}

// SerfQueryParam is provided to customize various serf query
// settings.
type SerfQueryParam struct {
	FilterNodes []string            // A list of node names to restrict query to
	FilterTags  map[string]string   // A map of tag name to regex to filter on
	RequestAck  bool                // Should nodes ack the query receipt
	Timeout     time.Duration       // Maximum query duration. Optional, will be set automatically.
	Name        string              // Opaque query name
	Payload     []byte              // Opaque query payload
	AckCh       chan<- string       // Channel to send Ack replies on
	RespCh      chan<- NodeResponse // Channel to send responses on
}

// SerfDiag handles Serf-based diagnostics for both the Consul server
// and the Consul client
type SerfDiag struct {
	// Logger uses the provided LogOutput
	logger *log.Logger

	// serf is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serf *serf.Serf
}

func NewSerfDiag(config *Config, serf *serf.Serf) (*SerfDiag, error) {
	// Check the protocol version
	if err := config.CheckVersion(); err != nil {
		return nil, err
	}

	// Ensure we have a log output
	if config.LogOutput == nil {
		config.LogOutput = os.Stderr
	}

	// Create a logger
	logger := log.New(config.LogOutput, "", log.LstdFlags)

	// Create the SerfDiag object.
	sd := &SerfDiag{
		logger: logger,
		serf:   serf,
	}

	return sd, nil
}

func (c *SerfDiag) Query(name string, payload []byte, params *serf.QueryParam) (*serf.QueryResponse, error) {
	// Prevent the use of the internal prefix
	if strings.HasPrefix(name, serf.InternalQueryPrefix) {
		// Allow the special "ping" query
		if name != serf.InternalQueryPrefix+"ping" || payload != nil {
			return nil, fmt.Errorf("Serf Queries cannot contain the '%s' prefix", serf.InternalQueryPrefix)
		}
	}
	c.logger.Printf("[DEBUG] client: Requesting serf query send: %s. Payload: %#v",
		name, string(payload))
	resp, err := c.serf.Query(name, payload, params)
	if err != nil {
		c.logger.Printf("[WARN] client: failed to start user serf query: %v", err)
	}
	return resp, err
}

// SerfPingParam is provided to customize various Serf Ping settings.
type SerfPingParam struct {
	Name string // Name of node to ping
}

type SerfPingResponse struct {
	Success bool
	RTT     time.Duration
}

func (c *SerfDiag) Ping(params *SerfPingParam) (*SerfPingResponse, error) {
	c.logger.Printf("[DEBUG] client: Requesting serf ping send: %s", params.Name)
	var node *serf.Member
	node = nil
	for _, m := range c.serf.Members() {
		if m.Name == params.Name {
			node = &m
			break
		}
	}
	if node == nil {
		return nil, fmt.Errorf("Member %s not found in data center.", params.Name)
	}
	if rtt, err := c.serf.Memberlist().Ping(node.Name, node.Addr, node.Port); err == nil {
		return &SerfPingResponse{
			Success: true,
			RTT:     *rtt,
		}, err
	} else {
		switch err.(type) {
		case memberlist.NoPingResponseError:
			return &SerfPingResponse{
				Success: false,
				RTT:     *rtt,
			}, nil
		default:
			return nil, err
		}
	}
}
