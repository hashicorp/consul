package consul

import (
	"fmt"
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

// SerfQuery handles 'reachability' Serf 'pings' for both the client
// and server.
type SerfQuery struct {
	// Logger uses the provided LogOutput
	logger *log.Logger

	// serf is the Serf cluster maintained inside the DC
	// which contains all the DC nodes
	serf *serf.Serf
}

func NewSerfQuery(config *Config, serf *serf.Serf) (*SerfQuery, error) {
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

	// Create the SerfQuery object.
	sq := &SerfQuery{
		logger: logger,
		serf:   serf,
	}

	return sq, nil
}

func (c *SerfQuery) Query(name string, payload []byte, params *serf.QueryParam) (*serf.QueryResponse, error) {
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
