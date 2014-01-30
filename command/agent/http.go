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

// NewHTTPServer starts a new HTTP server to provide an interface to
// the agent.
func NewHTTPServer(agent *Agent, logOutput io.Writer, bind string) (*HTTPServer, error) {
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
	s.mux.HandleFunc("/", s.Index)

	s.mux.HandleFunc("/v1/status/leader", s.wrap(s.StatusLeader))
	s.mux.HandleFunc("/v1/status/peers", s.wrap(s.StatusPeers))

	s.mux.HandleFunc("/v1/catalog/register", s.wrap(s.CatalogRegister))
	s.mux.HandleFunc("/v1/catalog/deregister", s.wrap(s.CatalogDeregister))
	s.mux.HandleFunc("/v1/catalog/datacenters", s.wrap(s.CatalogDatacenters))
	s.mux.HandleFunc("/v1/catalog/nodes", s.wrap(s.CatalogNodes))
	s.mux.HandleFunc("/v1/catalog/services", s.wrap(s.CatalogServices))
	s.mux.HandleFunc("/v1/catalog/service/", s.wrap(s.CatalogServiceNodes))
	s.mux.HandleFunc("/v1/catalog/node/", s.wrap(s.CatalogNodeServices))

	s.mux.HandleFunc("/v1/health/node/", s.wrap(s.HealthNodeChecks))
	s.mux.HandleFunc("/v1/health/checks/", s.wrap(s.HealthServiceChecks))
	s.mux.HandleFunc("/v1/health/state/", s.wrap(s.HealthChecksInState))
	s.mux.HandleFunc("/v1/health/service/", s.wrap(s.HealthServiceNodes))

	s.mux.HandleFunc("/v1/agent/services", s.wrap(s.AgentServices))
	s.mux.HandleFunc("/v1/agent/checks", s.wrap(s.AgentChecks))
	s.mux.HandleFunc("/v1/agent/members", s.wrap(s.AgentMembers))
	s.mux.HandleFunc("/v1/agent/join/", s.wrap(s.AgentJoin))
	s.mux.HandleFunc("/v1/agent/force-leave/", s.wrap(s.AgentForceLeave))

	s.mux.HandleFunc("/v1/agent/check/register", s.wrap(s.AgentRegisterCheck))
	s.mux.HandleFunc("/v1/agent/check/deregister", s.wrap(s.AgentDeregisterCheck))
	s.mux.HandleFunc("/v1/agent/check/pass/", s.wrap(s.AgentCheckPass))
	s.mux.HandleFunc("/v1/agent/check/warn/", s.wrap(s.AgentCheckWarn))
	s.mux.HandleFunc("/v1/agent/check/fail/", s.wrap(s.AgentCheckFail))

	s.mux.HandleFunc("/v1/agent/service/register", s.wrap(s.AgentRegisterService))
	s.mux.HandleFunc("/v1/agent/service/deregister", s.wrap(s.AgentDeregisterService))
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		// Invoke the handler
		start := time.Now()
		defer func() {
			s.logger.Printf("[DEBUG] http: Request %v (%v)", req.URL, time.Now().Sub(start))
		}()
		obj, err := handler(resp, req)

		// Check for an error
	HAS_ERR:
		if err != nil {
			s.logger.Printf("[ERR] http: Request %v, error: %v", req.URL, err)
			resp.WriteHeader(500)
			resp.Write([]byte(err.Error()))
			return
		}

		// Write out the JSON object
		if obj != nil {
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			if err = enc.Encode(obj); err != nil {
				goto HAS_ERR
			}
			resp.Write(buf.Bytes())
		}
	}
	return f
}

// Renders a simple index page
func (s *HTTPServer) Index(resp http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		resp.Write([]byte("Consul Agent"))
	} else {
		resp.WriteHeader(404)
	}
}

// decodeBody is used to decode a JSON request body
func decodeBody(req *http.Request, out interface{}) error {
	dec := json.NewDecoder(req.Body)
	return dec.Decode(out)
}
