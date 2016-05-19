package agent

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/mitchellh/mapstructure"
)

var (
	// scadaHTTPAddr is the address associated with the
	// HTTPServer. When populating an ACL token for a request,
	// this is checked to switch between the ACLToken and
	// AtlasACLToken
	scadaHTTPAddr = "SCADA"
)

// HTTPServer is used to wrap an Agent and expose various API's
// in a RESTful manner
type HTTPServer struct {
	agent    *Agent
	mux      *http.ServeMux
	listener net.Listener
	logger   *log.Logger
	uiDir    string
	addr     string
}

// acceptedMethods is a map from a path (e.g. "/v1/kv") to a comma-delimited
// list of HTTP verbs accepted at that endpoint (e.g. "GET,PUT,DELETE").
var acceptedMethods = make(map[string]string)

// NewHTTPServers starts new HTTP servers to provide an interface to
// the agent.
func NewHTTPServers(agent *Agent, config *Config, logOutput io.Writer) ([]*HTTPServer, error) {
	var servers []*HTTPServer

	if config.Ports.HTTPS > 0 {
		httpAddr, err := config.ClientListener(config.Addresses.HTTPS, config.Ports.HTTPS)
		if err != nil {
			return nil, err
		}

		tlsConf := &tlsutil.Config{
			VerifyIncoming: config.VerifyIncoming,
			VerifyOutgoing: config.VerifyOutgoing,
			CAFile:         config.CAFile,
			CertFile:       config.CertFile,
			KeyFile:        config.KeyFile,
			NodeName:       config.NodeName,
			ServerName:     config.ServerName}

		tlsConfig, err := tlsConf.IncomingTLSConfig()
		if err != nil {
			return nil, err
		}

		ln, err := net.Listen(httpAddr.Network(), httpAddr.String())
		if err != nil {
			return nil, fmt.Errorf("Failed to get Listen on %s: %v", httpAddr.String(), err)
		}

		list := tls.NewListener(tcpKeepAliveListener{ln.(*net.TCPListener)}, tlsConfig)

		// Create the mux
		mux := http.NewServeMux()

		// Create the server
		srv := &HTTPServer{
			agent:    agent,
			mux:      mux,
			listener: list,
			logger:   log.New(logOutput, "", log.LstdFlags),
			uiDir:    config.UiDir,
			addr:     httpAddr.String(),
		}
		srv.registerHandlers(config.EnableDebug)

		// Start the server
		go http.Serve(list, mux)
		servers = append(servers, srv)
	}

	if config.Ports.HTTP > 0 {
		httpAddr, err := config.ClientListener(config.Addresses.HTTP, config.Ports.HTTP)
		if err != nil {
			return nil, fmt.Errorf("Failed to get ClientListener address:port: %v", err)
		}

		// Error if we are trying to bind a domain socket to an existing path
		socketPath, isSocket := unixSocketAddr(config.Addresses.HTTP)
		if isSocket {
			if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
				agent.logger.Printf("[WARN] agent: Replacing socket %q", socketPath)
			}
			if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
				return nil, fmt.Errorf("error removing socket file: %s", err)
			}
		}

		ln, err := net.Listen(httpAddr.Network(), httpAddr.String())
		if err != nil {
			return nil, fmt.Errorf("Failed to get Listen on %s: %v", httpAddr.String(), err)
		}

		var list net.Listener
		if isSocket {
			// Set up ownership/permission bits on the socket file
			if err := setFilePermissions(socketPath, config.UnixSockets); err != nil {
				return nil, fmt.Errorf("Failed setting up HTTP socket: %s", err)
			}
			list = ln
		} else {
			list = tcpKeepAliveListener{ln.(*net.TCPListener)}
		}

		// Create the mux
		mux := http.NewServeMux()

		// Create the server
		srv := &HTTPServer{
			agent:    agent,
			mux:      mux,
			listener: list,
			logger:   log.New(logOutput, "", log.LstdFlags),
			uiDir:    config.UiDir,
			addr:     httpAddr.String(),
		}
		srv.registerHandlers(config.EnableDebug)

		// Start the server
		go http.Serve(list, mux)
		servers = append(servers, srv)
	}

	return servers, nil
}

// newScadaHttp creates a new HTTP server wrapping the SCADA
// listener such that HTTP calls can be sent from the brokers.
func newScadaHttp(agent *Agent, list net.Listener) *HTTPServer {
	// Create the mux
	mux := http.NewServeMux()

	// Create the server
	srv := &HTTPServer{
		agent:    agent,
		mux:      mux,
		listener: list,
		logger:   agent.logger,
		addr:     scadaHTTPAddr,
	}
	srv.registerHandlers(false) // Never allow debug for SCADA

	// Start the server
	go http.Serve(list, mux)
	return srv
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by NewHttpServer so
// dead TCP connections eventually go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(30 * time.Second)
	return tc, nil
}

// Shutdown is used to shutdown the HTTP server
func (s *HTTPServer) Shutdown() {
	if s != nil {
		s.logger.Printf("[DEBUG] http: Shutting down http server (%v)", s.addr)
		s.listener.Close()
	}
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
	register("/", "GET", s.Index)

	register("/v1/status/leader", "GET", s.wrap(s.StatusLeader))
	register("/v1/status/peers", "GET", s.wrap(s.StatusPeers))

	register("/v1/catalog/register", "PUT", s.wrap(s.CatalogRegister))
	register("/v1/catalog/deregister", "PUT", s.wrap(s.CatalogDeregister))
	register("/v1/catalog/datacenters", "GET", s.wrap(s.CatalogDatacenters))
	register("/v1/catalog/nodes", "GET", s.wrap(s.CatalogNodes))
	register("/v1/catalog/services", "GET", s.wrap(s.CatalogServices))
	register("/v1/catalog/service/", "GET", s.wrap(s.CatalogServiceNodes))
	register("/v1/catalog/node/", "GET", s.wrap(s.CatalogNodeServices))

	if !s.agent.config.DisableCoordinates {
		register("/v1/coordinate/datacenters", "GET", s.wrap(s.CoordinateDatacenters))
		register("/v1/coordinate/nodes", "GET", s.wrap(s.CoordinateNodes))
	} else {
		register("/v1/coordinate/datacenters", "GET", s.wrap(coordinateDisabled))
		register("/v1/coordinate/nodes", "GET", s.wrap(coordinateDisabled))
	}

	register("/v1/health/node/", "GET", s.wrap(s.HealthNodeChecks))
	register("/v1/health/checks/", "GET", s.wrap(s.HealthServiceChecks))
	register("/v1/health/state/", "GET", s.wrap(s.HealthChecksInState))
	register("/v1/health/service/", "GET", s.wrap(s.HealthServiceNodes))

	register("/v1/agent/self", "GET", s.wrap(s.AgentSelf))
	register("/v1/agent/maintenance", "GET", s.wrap(s.AgentNodeMaintenance))
	register("/v1/agent/services", "GET", s.wrap(s.AgentServices))
	register("/v1/agent/checks", "GET", s.wrap(s.AgentChecks))
	register("/v1/agent/members", "GET", s.wrap(s.AgentMembers))
	register("/v1/agent/join/", "GET", s.wrap(s.AgentJoin))
	register("/v1/agent/force-leave/", "GET", s.wrap(s.AgentForceLeave))

	register("/v1/agent/check/register", "PUT", s.wrap(s.AgentRegisterCheck))
	register("/v1/agent/check/deregister/", "PUT", s.wrap(s.AgentDeregisterCheck))
	register("/v1/agent/check/pass/", "GET", s.wrap(s.AgentCheckPass))
	register("/v1/agent/check/warn/", "GET", s.wrap(s.AgentCheckWarn))
	register("/v1/agent/check/fail/", "GET", s.wrap(s.AgentCheckFail))
	register("/v1/agent/check/update/", "GET", s.wrap(s.AgentCheckUpdate))

	register("/v1/agent/service/register", "PUT", s.wrap(s.AgentRegisterService))
	register("/v1/agent/service/deregister/", "GET", s.wrap(s.AgentDeregisterService))
	register("/v1/agent/service/maintenance/", "GET", s.wrap(s.AgentServiceMaintenance))

	register("/v1/event/fire/", "PUT", s.wrap(s.EventFire))
	register("/v1/event/list", "GET", s.wrap(s.EventList))

	register("/v1/kv/", "GET,PUT,DELETE", s.wrap(s.KVSEndpoint))

	register("/v1/session/create", "PUT", s.wrap(s.SessionCreate))
	register("/v1/session/destroy/", "PUT", s.wrap(s.SessionDestroy))
	register("/v1/session/renew/", "PUT", s.wrap(s.SessionRenew))
	register("/v1/session/info/", "GET", s.wrap(s.SessionGet))
	register("/v1/session/node/", "GET", s.wrap(s.SessionsForNode))
	register("/v1/session/list", "GET", s.wrap(s.SessionList))

	if s.agent.config.ACLDatacenter != "" {
		register("/v1/acl/create", "PUT", s.wrap(s.ACLCreate))
		register("/v1/acl/update", "PUT", s.wrap(s.ACLUpdate))
		register("/v1/acl/destroy/", "PUT", s.wrap(s.ACLDestroy))
		register("/v1/acl/info/", "GET", s.wrap(s.ACLGet))
		register("/v1/acl/clone/", "PUT", s.wrap(s.ACLClone))
		register("/v1/acl/list", "GET", s.wrap(s.ACLList))
	} else {
		register("/v1/acl/create", "PUT", s.wrap(aclDisabled))
		register("/v1/acl/update", "PUT", s.wrap(aclDisabled))
		register("/v1/acl/destroy/", "PUT", s.wrap(aclDisabled))
		register("/v1/acl/info/", "GET", s.wrap(aclDisabled))
		register("/v1/acl/clone/", "PUT", s.wrap(aclDisabled))
		register("/v1/acl/list", "GET", s.wrap(aclDisabled))
	}

	register("/v1/query", "GET,POST", s.wrap(s.PreparedQueryGeneral))
	register("/v1/query/", "GET,PUT,DELETE", s.wrap(s.PreparedQuerySpecific))

	register("/v1/txn", "GET", s.wrap(s.Txn))

	if enableDebug {
		register("/debug/pprof/", "GET", pprof.Index)
		register("/debug/pprof/cmdline", "GET", pprof.Cmdline)
		register("/debug/pprof/profile", "GET", pprof.Profile)
		register("/debug/pprof/symbol", "GET", pprof.Symbol)
	}

	// Use the custom UI dir if provided.
	if s.uiDir != "" {
		s.mux.Handle("/ui/", "GET", http.StripPrefix("/ui/", http.FileServer(http.Dir(s.uiDir))))
	} else if s.agent.config.EnableUi {
		s.mux.Handle("/ui/", "GET", http.StripPrefix("/ui/", http.FileServer(assetFS())))
	}

	// API's are under /internal/ui/ to avoid conflict
	register("/v1/internal/ui/nodes", "GET", s.wrap(s.UINodes))
	register("/v1/internal/ui/node/", "GET", s.wrap(s.UINodeInfo))
	register("/v1/internal/ui/services", "GET", s.wrap(s.UIServices))
}

// register adds a path and a handler to the ServeMux, and adds the endpoint's
// accepted HTTP verbs to acceptedMethods
func (s *HTTPServer) register(path, methods string, handler func(resp http.ResponseWriter, req *http.Request)) {
	acceptedMethods[path] = methods
	s.mux.HandleFunc(path, handler)
}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.config.HTTPAPIResponseHeaders)

		// Obfuscate any tokens from appearing in the logs
		formVals, err := url.ParseQuery(req.URL.RawQuery)
		if err != nil {
			s.logger.Printf("[ERR] http: Failed to decode query: %s from=%s", err, req.RemoteAddr)
			resp.WriteHeader(http.StatusInternalServerError) // 500
			return
		}
		logURL := req.URL.String()
		if tokens, ok := formVals["token"]; ok {
			for _, token := range tokens {
				if token == "" {
					logURL += "<hidden>"
					continue
				}
				logURL = strings.Replace(logURL, token, "<hidden>", -1)
			}
		}

		// TODO (slackpad) We may want to consider redacting prepared
		// query names/IDs here since they are proxies for tokens. But,
		// knowing one only gives you read access to service listings
		// which is pretty trivial, so it's probably not worth the code
		// complexity and overhead of filtering them out. You can't
		// recover the token it's a proxy for with just the query info;
		// you'd need the actual token (or a management token) to read
		// that back.

		start := time.Now()
		defer func() {
			s.logger.Printf("[DEBUG] http: Request %s %v (%v) from=%s", req.Method, logURL, time.Now().Sub(start), req.RemoteAddr)
		}()

		var obj interface{}

		// respond appropriately to OPTIONS requests
		if req.Method == "OPTIONS" {
			for prefix, methods := range allowedMethods {
				if strings.HasPrefix(req.URL.Path, prefix) {
					resp.Header.Set("Allow", "OPTIONS,"+methods)
					obj, err = nil, nil
				}
			}
			resp.WriteHeader(http.StatusNotFound) // 404
			return
		}

		// Invoke the handler
		obj, err = handler(resp, req)

		// Check for an error
	HAS_ERR:
		if err != nil {
			s.logger.Printf("[ERR] http: Request %s %v, error: %v from=%s", req.Method, logURL, err, req.RemoteAddr)
			code := http.StatusInternalServerError // 500
			errMsg := err.Error()
			if strings.Contains(errMsg, "Permission denied") || strings.Contains(errMsg, "ACL not found") {
				code = http.StatusForbidden // 403
			}
			resp.WriteHeader(code)
			resp.Write([]byte(err.Error()))
			return
		}

		if obj != nil {
			var buf []byte
			buf, err = s.marshalJSON(req, obj)
			if err != nil {
				goto HAS_ERR
			}

			resp.Header().Set("Content-Type", "application/json")
			resp.Write(buf)
		}
	}
	return f
}

// marshalJSON marshals the object into JSON, respecting the user's pretty-ness
// configuration.
func (s *HTTPServer) marshalJSON(req *http.Request, obj interface{}) ([]byte, error) {
	if _, ok := req.URL.Query()["pretty"]; ok {
		buf, err := json.MarshalIndent(obj, "", "    ")
		if err != nil {
			return nil, err
		}
		buf = append(buf, "\n"...)
		return buf, nil
	}

	buf, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return buf, err
}

// Returns true if the UI is enabled.
func (s *HTTPServer) IsUIEnabled() bool {
	return s.uiDir != "" || s.agent.config.EnableUi
}

// Renders a simple index page
func (s *HTTPServer) Index(resp http.ResponseWriter, req *http.Request) {
	// Check if this is a non-index path
	if req.URL.Path != "/" {
		resp.WriteHeader(http.StatusNotFound) // 404
		return
	}

	// Give them something helpful if there's no UI so they at least know
	// what this server is.
	if !s.IsUIEnabled() {
		resp.Write([]byte("Consul Agent"))
		return
	}

	// Redirect to the UI endpoint
	http.Redirect(resp, req, "/ui/", http.StatusMovedPermanently) // 301
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
	resp.Header().Set("X-Consul-Index", strconv.FormatUint(index, 10))
}

// setKnownLeader is used to set the known leader header
func setKnownLeader(resp http.ResponseWriter, known bool) {
	s := "true"
	if !known {
		s = "false"
	}
	resp.Header().Set("X-Consul-KnownLeader", s)
}

// setLastContact is used to set the last contact header
func setLastContact(resp http.ResponseWriter, last time.Duration) {
	lastMsec := uint64(last / time.Millisecond)
	resp.Header().Set("X-Consul-LastContact", strconv.FormatUint(lastMsec, 10))
}

// setMeta is used to set the query response meta data
func setMeta(resp http.ResponseWriter, m *structs.QueryMeta) {
	setIndex(resp, m.Index)
	setLastContact(resp, m.LastContact)
	setKnownLeader(resp, m.KnownLeader)
}

// setHeaders is used to set canonical response header fields
func setHeaders(resp http.ResponseWriter, headers map[string]string) {
	for field, value := range headers {
		resp.Header().Set(http.CanonicalHeaderKey(field), value)
	}
}

// parseWait is used to parse the ?wait and ?index query params
// Returns true on error
func parseWait(resp http.ResponseWriter, req *http.Request, b *structs.QueryOptions) bool {
	query := req.URL.Query()
	if wait := query.Get("wait"); wait != "" {
		dur, err := time.ParseDuration(wait)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest) // 400
			resp.Write([]byte("Invalid wait time"))
			return true
		}
		b.MaxQueryTime = dur
	}
	if idx := query.Get("index"); idx != "" {
		index, err := strconv.ParseUint(idx, 10, 64)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest) // 400
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
		resp.WriteHeader(http.StatusBadRequest) // 400
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

// parseToken is used to parse the ?token query param or the X-Consul-Token header
func (s *HTTPServer) parseToken(req *http.Request, token *string) {
	if other := req.URL.Query().Get("token"); other != "" {
		*token = other
		return
	}

	if other := req.Header.Get("X-Consul-Token"); other != "" {
		*token = other
		return
	}

	// Set the AtlasACLToken if SCADA
	if s.addr == scadaHTTPAddr && s.agent.config.AtlasACLToken != "" {
		*token = s.agent.config.AtlasACLToken
		return
	}

	// Set the default ACLToken
	*token = s.agent.config.ACLToken
}

// parseSource is used to parse the ?near=<node> query parameter, used for
// sorting by RTT based on a source node. We set the source's DC to the target
// DC in the request, if given, or else the agent's DC.
func (s *HTTPServer) parseSource(req *http.Request, source *structs.QuerySource) {
	s.parseDC(req, &source.Datacenter)
	if node := req.URL.Query().Get("near"); node != "" {
		if node == "_agent" {
			source.Node = s.agent.config.NodeName
		} else {
			source.Node = node
		}
	}
}

// parse is a convenience method for endpoints that need
// to use both parseWait and parseDC.
func (s *HTTPServer) parse(resp http.ResponseWriter, req *http.Request, dc *string, b *structs.QueryOptions) bool {
	s.parseDC(req, dc)
	s.parseToken(req, &b.Token)
	if parseConsistency(resp, req, b) {
		return true
	}
	return parseWait(resp, req, b)
}
