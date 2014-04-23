package agent

import (
	"bytes"
	"encoding/json"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/mitchellh/mapstructure"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"
)

// HTTPServer is used to wrap an Agent and expose various API's
// in a RESTful manner
type HTTPServer struct {
	agent    *Agent
	mux      *http.ServeMux
	listener net.Listener
	logger   *log.Logger
	uiDir    string
}

// NewHTTPServer starts a new HTTP server to provide an interface to
// the agent.
func NewHTTPServer(agent *Agent, uiDir string, enableDebug bool, logOutput io.Writer, bind string) (*HTTPServer, error) {
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
	srv.registerHandlers(enableDebug)

	// Start the server
	go http.Serve(list, mux)
	return srv, nil
}

// Shutdown is used to shutdown the HTTP server
func (s *HTTPServer) Shutdown() {
	s.listener.Close()
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
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
	s.mux.HandleFunc("/v1/agent/check/deregister/", s.wrap(s.AgentDeregisterCheck))
	s.mux.HandleFunc("/v1/agent/check/pass/", s.wrap(s.AgentCheckPass))
	s.mux.HandleFunc("/v1/agent/check/warn/", s.wrap(s.AgentCheckWarn))
	s.mux.HandleFunc("/v1/agent/check/fail/", s.wrap(s.AgentCheckFail))

	s.mux.HandleFunc("/v1/agent/service/register", s.wrap(s.AgentRegisterService))
	s.mux.HandleFunc("/v1/agent/service/deregister/", s.wrap(s.AgentDeregisterService))

	s.mux.HandleFunc("/v1/kv/", s.wrap(s.KVSEndpoint))

	if enableDebug {
		s.mux.HandleFunc("/debug/pprof/", pprof.Index)
		s.mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		s.mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		s.mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	}

	// Enable the UI + special endpoints
	if s.uiDir != "" {
		s.mux.HandleFunc("/ui/", s.UiIndex)
	}
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
			resp.Header().Set("Content-Type", "application/json")
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
func decodeBody(req *http.Request, out interface{}, cb func(interface{}) error) error {
	var raw interface{}
	dec := json.NewDecoder(req.Body)
	if err := dec.Decode(&raw); err != nil {
		return err
	}

	// Invoke the callback prior to decode
	if cb != nil {
		if err := cb(raw); err != nil {
			return err
		}
	}
	return mapstructure.Decode(raw, out)
}

// setIndex is used to set the index response header
func setIndex(resp http.ResponseWriter, index uint64) {
	resp.Header().Add("X-Consul-Index", strconv.FormatUint(index, 10))
}

// setKnownLeader is used to set the known leader header
func setKnownLeader(resp http.ResponseWriter, known bool) {
	s := "true"
	if !known {
		s = "false"
	}
	resp.Header().Add("X-Consul-KnownLeader", s)
}

// setLastContact is used to set the last contact header
func setLastContact(resp http.ResponseWriter, last time.Duration) {
	lastMsec := uint64(last / time.Millisecond)
	resp.Header().Add("X-Consul-LastContact", strconv.FormatUint(lastMsec, 10))
}

// setMeta is used to set the query response meta data
func setMeta(resp http.ResponseWriter, m *structs.QueryMeta) {
	setIndex(resp, m.Index)
	setLastContact(resp, m.LastContact)
	setKnownLeader(resp, m.KnownLeader)
}

// parseWait is used to parse the ?wait and ?index query params
// Returns true on error
func parseWait(resp http.ResponseWriter, req *http.Request, b *structs.QueryOptions) bool {
	query := req.URL.Query()
	if wait := query.Get("wait"); wait != "" {
		dur, err := time.ParseDuration(wait)
		if err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte("Invalid wait time"))
			return true
		}
		b.MaxQueryTime = dur
	}
	if idx := query.Get("index"); idx != "" {
		index, err := strconv.ParseUint(idx, 10, 64)
		if err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte("Invalid index"))
			return true
		}
		b.MinQueryIndex = index
	}
	return false
}

// parseConsistency is used to parse the ?stale and ?consistent query params.
// Returns true on error
func parseConsistency(resp http.ResponseWriter, req *http.Request, b *structs.QueryOptions) bool {
	query := req.URL.Query()
	if _, ok := query["stale"]; ok {
		b.AllowStale = true
	}
	if _, ok := query["consistent"]; ok {
		b.RequireConsistent = true
	}
	if b.AllowStale && b.RequireConsistent {
		resp.WriteHeader(400)
		resp.Write([]byte("Cannot specify ?stale with ?consistent, conflicting semantics."))
		return true
	}
	return false
}

// parseDC is used to parse the ?dc query param
func (s *HTTPServer) parseDC(req *http.Request, dc *string) {
	if other := req.URL.Query().Get("dc"); other != "" {
		*dc = other
	} else if *dc == "" {
		*dc = s.agent.config.Datacenter
	}
}

// parse is a convenience method for endpoints that need
// to use both parseWait and parseDC.
func (s *HTTPServer) parse(resp http.ResponseWriter, req *http.Request, dc *string, b *structs.QueryOptions) bool {
	s.parseDC(req, dc)
	if parseConsistency(resp, req, b) {
		return true
	}
	return parseWait(resp, req, b)
}
