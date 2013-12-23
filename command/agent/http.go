package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// HTTPServer is used to wrap an Agent and expose various API's
// in a RESTful manner
type HTTPServer struct {
	agent    *Agent
	mux      *http.ServeMux
	listener net.Listener
	logger   *log.Logger
}

// NewServer starts a new HTTP server to provide an interface to
// the agent.
func NewServer(agent *Agent, logOutput io.Writer, bind string) (*HTTPServer, error) {
	// Create the mux
	mux := http.NewServeMux()

	// Create listener
	list, err := net.Listen("tcp", bind)
	if err != nil {
		return nil, err
	}

	// Create the server
	srv := &HTTPServer{
		agent:    agent,
		mux:      mux,
		listener: list,
		logger:   log.New(logOutput, "", log.LstdFlags),
	}
	srv.registerHandlers()

	// Start the server
	go http.Serve(list, mux)
	return srv, nil
}

// Shutdown is used to shutdown the HTTP server
func (s *HTTPServer) Shutdown() {
	s.listener.Close()
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers() {
	s.mux.HandleFunc("/v1/status/leader", s.wrap(s.StatusLeader))
	s.mux.HandleFunc("/v1/status/peers", s.wrap(s.StatusPeers))

	s.mux.HandleFunc("/v1/catalog/datacenters", s.wrap(s.CatalogDatacenters))
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		// Invoke the handler
		start := time.Now()
		defer func() {
			s.logger.Printf("[DEBUG] HTTP Request %v (%v)", req.URL, time.Now().Sub(start))
		}()
		obj, err := handler(req)

		// Check for an error
	HAS_ERR:
		if err != nil {
			s.logger.Printf("[ERR] Request %v, error: %v", req, err)
			resp.WriteHeader(500)
			resp.Write([]byte(err.Error()))
			return
		}

		// Write out the JSON object
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		if err = enc.Encode(obj); err != nil {
			goto HAS_ERR
		}
		resp.Write(buf.Bytes())
	}
	return f
}
