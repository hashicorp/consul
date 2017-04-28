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

	"github.com/armon/go-metrics"
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

// NewHTTPServers starts new HTTP servers to provide an interface to
// the agent.
func NewHTTPServers(agent *Agent, config *Config, logOutput io.Writer) ([]*HTTPServer, error) {
	if logOutput == nil {
		return nil, fmt.Errorf("Please provide a valid logOutput(io.Writer)")
	}

	var servers []*HTTPServer

	if config.Ports.HTTPS > 0 {
		httpAddr, err := config.ClientListener(config.Addresses.HTTPS, config.Ports.HTTPS)
		if err != nil {
			return nil, err
		}

		tlsConf := &tlsutil.Config{
			VerifyIncoming:           config.VerifyIncoming || config.VerifyIncomingHTTPS,
			VerifyOutgoing:           config.VerifyOutgoing,
			CAFile:                   config.CAFile,
			CAPath:                   config.CAPath,
			CertFile:                 config.CertFile,
			KeyFile:                  config.KeyFile,
			NodeName:                 config.NodeName,
			ServerName:               config.ServerName,
			TLSMinVersion:            config.TLSMinVersion,
			CipherSuites:             config.TLSCipherSuites,
			PreferServerCipherSuites: config.TLSPreferServerCipherSuites,
		}

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
			uiDir:    config.UIDir,
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
			uiDir:    config.UIDir,
			addr:     httpAddr.String(),
		}
		srv.registerHandlers(config.EnableDebug)

		// Start the server
		go http.Serve(list, mux)
		servers = append(servers, srv)
	}

	return servers, nil
}

// newScadaHTTP creates a new HTTP server wrapping the SCADA
// listener such that HTTP calls can be sent from the brokers.
func newScadaHTTP(agent *Agent, list net.Listener) *HTTPServer {
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

// handleFuncMetrics takes the given pattern and handler and wraps to produce
// metrics based on the pattern and request.
func (s *HTTPServer) handleFuncMetrics(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	// Get the parts of the pattern. We omit any initial empty for the
	// leading slash, and put an underscore as a "thing" placeholder if we
	// see a trailing slash, which means the part after is parsed. This lets
	// us distinguish from things like /v1/query and /v1/query/<query id>.
	var parts []string
	for i, part := range strings.Split(pattern, "/") {
		if part == "" {
			if i == 0 {
				continue
			} else {
				part = "_"
			}
		}
		parts = append(parts, part)
	}

	// Register the wrapper, which will close over the expensive-to-compute
	// parts from above.
	wrapper := func(resp http.ResponseWriter, req *http.Request) {
		start := time.Now()
		handler(resp, req)

		key := append([]string{"consul", "http", req.Method}, parts...)
		metrics.MeasureSince(key, start)
	}
	s.mux.HandleFunc(pattern, wrapper)
}

// registerHandlers is used to attach our handlers to the mux
func (s *HTTPServer) registerHandlers(enableDebug bool) {
	s.mux.HandleFunc("/", s.Index)

	// API V1.
	if s.agent.config.ACLDatacenter != "" {
		s.handleFuncMetrics("/v1/acl/create", s.wrap(s.ACLCreate))
		s.handleFuncMetrics("/v1/acl/update", s.wrap(s.ACLUpdate))
		s.handleFuncMetrics("/v1/acl/destroy/", s.wrap(s.ACLDestroy))
		s.handleFuncMetrics("/v1/acl/info/", s.wrap(s.ACLGet))
		s.handleFuncMetrics("/v1/acl/clone/", s.wrap(s.ACLClone))
		s.handleFuncMetrics("/v1/acl/list", s.wrap(s.ACLList))
		s.handleFuncMetrics("/v1/acl/replication", s.wrap(s.ACLReplicationStatus))
	} else {
		s.handleFuncMetrics("/v1/acl/create", s.wrap(ACLDisabled))
		s.handleFuncMetrics("/v1/acl/update", s.wrap(ACLDisabled))
		s.handleFuncMetrics("/v1/acl/destroy/", s.wrap(ACLDisabled))
		s.handleFuncMetrics("/v1/acl/info/", s.wrap(ACLDisabled))
		s.handleFuncMetrics("/v1/acl/clone/", s.wrap(ACLDisabled))
		s.handleFuncMetrics("/v1/acl/list", s.wrap(ACLDisabled))
		s.handleFuncMetrics("/v1/acl/replication", s.wrap(ACLDisabled))
	}
	s.handleFuncMetrics("/v1/agent/self", s.wrap(s.AgentSelf))
	s.handleFuncMetrics("/v1/agent/maintenance", s.wrap(s.AgentNodeMaintenance))
	s.handleFuncMetrics("/v1/agent/reload", s.wrap(s.AgentReload))
	s.handleFuncMetrics("/v1/agent/monitor", s.wrap(s.AgentMonitor))
	s.handleFuncMetrics("/v1/agent/services", s.wrap(s.AgentServices))
	s.handleFuncMetrics("/v1/agent/checks", s.wrap(s.AgentChecks))
	s.handleFuncMetrics("/v1/agent/members", s.wrap(s.AgentMembers))
	s.handleFuncMetrics("/v1/agent/join/", s.wrap(s.AgentJoin))
	s.handleFuncMetrics("/v1/agent/leave", s.wrap(s.AgentLeave))
	s.handleFuncMetrics("/v1/agent/force-leave/", s.wrap(s.AgentForceLeave))
	s.handleFuncMetrics("/v1/agent/check/register", s.wrap(s.AgentRegisterCheck))
	s.handleFuncMetrics("/v1/agent/check/deregister/", s.wrap(s.AgentDeregisterCheck))
	s.handleFuncMetrics("/v1/agent/check/pass/", s.wrap(s.AgentCheckPass))
	s.handleFuncMetrics("/v1/agent/check/warn/", s.wrap(s.AgentCheckWarn))
	s.handleFuncMetrics("/v1/agent/check/fail/", s.wrap(s.AgentCheckFail))
	s.handleFuncMetrics("/v1/agent/check/update/", s.wrap(s.AgentCheckUpdate))
	s.handleFuncMetrics("/v1/agent/service/register", s.wrap(s.AgentRegisterService))
	s.handleFuncMetrics("/v1/agent/service/deregister/", s.wrap(s.AgentDeregisterService))
	s.handleFuncMetrics("/v1/agent/service/maintenance/", s.wrap(s.AgentServiceMaintenance))
	s.handleFuncMetrics("/v1/catalog/register", s.wrap(s.CatalogRegister))
	s.handleFuncMetrics("/v1/catalog/deregister", s.wrap(s.CatalogDeregister))
	s.handleFuncMetrics("/v1/catalog/datacenters", s.wrap(s.CatalogDatacenters))
	s.handleFuncMetrics("/v1/catalog/nodes", s.wrap(s.CatalogNodes))
	s.handleFuncMetrics("/v1/catalog/services", s.wrap(s.CatalogServices))
	s.handleFuncMetrics("/v1/catalog/service/", s.wrap(s.CatalogServiceNodes))
	s.handleFuncMetrics("/v1/catalog/node/", s.wrap(s.CatalogNodeServices))
	if !s.agent.config.DisableCoordinates {
		s.handleFuncMetrics("/v1/coordinate/datacenters", s.wrap(s.CoordinateDatacenters))
		s.handleFuncMetrics("/v1/coordinate/nodes", s.wrap(s.CoordinateNodes))
	} else {
		s.handleFuncMetrics("/v1/coordinate/datacenters", s.wrap(coordinateDisabled))
		s.handleFuncMetrics("/v1/coordinate/nodes", s.wrap(coordinateDisabled))
	}
	s.handleFuncMetrics("/v1/event/fire/", s.wrap(s.EventFire))
	s.handleFuncMetrics("/v1/event/list", s.wrap(s.EventList))
	s.handleFuncMetrics("/v1/health/node/", s.wrap(s.HealthNodeChecks))
	s.handleFuncMetrics("/v1/health/checks/", s.wrap(s.HealthServiceChecks))
	s.handleFuncMetrics("/v1/health/state/", s.wrap(s.HealthChecksInState))
	s.handleFuncMetrics("/v1/health/service/", s.wrap(s.HealthServiceNodes))
	s.handleFuncMetrics("/v1/internal/ui/nodes", s.wrap(s.UINodes))
	s.handleFuncMetrics("/v1/internal/ui/node/", s.wrap(s.UINodeInfo))
	s.handleFuncMetrics("/v1/internal/ui/services", s.wrap(s.UIServices))
	s.handleFuncMetrics("/v1/kv/", s.wrap(s.KVSEndpoint))
	s.handleFuncMetrics("/v1/operator/raft/configuration", s.wrap(s.OperatorRaftConfiguration))
	s.handleFuncMetrics("/v1/operator/raft/peer", s.wrap(s.OperatorRaftPeer))
	s.handleFuncMetrics("/v1/operator/keyring", s.wrap(s.OperatorKeyringEndpoint))
	s.handleFuncMetrics("/v1/operator/autopilot/configuration", s.wrap(s.OperatorAutopilotConfiguration))
	s.handleFuncMetrics("/v1/operator/autopilot/health", s.wrap(s.OperatorServerHealth))
	s.handleFuncMetrics("/v1/query", s.wrap(s.PreparedQueryGeneral))
	s.handleFuncMetrics("/v1/query/", s.wrap(s.PreparedQuerySpecific))
	s.handleFuncMetrics("/v1/session/create", s.wrap(s.SessionCreate))
	s.handleFuncMetrics("/v1/session/destroy/", s.wrap(s.SessionDestroy))
	s.handleFuncMetrics("/v1/session/renew/", s.wrap(s.SessionRenew))
	s.handleFuncMetrics("/v1/session/info/", s.wrap(s.SessionGet))
	s.handleFuncMetrics("/v1/session/node/", s.wrap(s.SessionsForNode))
	s.handleFuncMetrics("/v1/session/list", s.wrap(s.SessionList))
	s.handleFuncMetrics("/v1/status/leader", s.wrap(s.StatusLeader))
	s.handleFuncMetrics("/v1/status/peers", s.wrap(s.StatusPeers))
	s.handleFuncMetrics("/v1/snapshot", s.wrap(s.Snapshot))
	s.handleFuncMetrics("/v1/txn", s.wrap(s.Txn))

	// Debug endpoints.
	if enableDebug {
		s.handleFuncMetrics("/debug/pprof/", pprof.Index)
		s.handleFuncMetrics("/debug/pprof/cmdline", pprof.Cmdline)
		s.handleFuncMetrics("/debug/pprof/profile", pprof.Profile)
		s.handleFuncMetrics("/debug/pprof/symbol", pprof.Symbol)
	}

	// Use the custom UI dir if provided.
	if s.uiDir != "" {
		s.mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir(s.uiDir))))
	} else if s.agent.config.EnableUI {
		s.mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(assetFS())))
	}

}

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) func(resp http.ResponseWriter, req *http.Request) {
	f := func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.config.HTTPAPIResponseHeaders)
		setTranslateAddr(resp, s.agent.config.TranslateWanAddrs)

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

		// Invoke the handler
		start := time.Now()
		defer func() {
			s.logger.Printf("[DEBUG] http: Request %s %v (%v) from=%s", req.Method, logURL, time.Now().Sub(start), req.RemoteAddr)
		}()
		obj, err := handler(resp, req)

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
			fmt.Fprint(resp, err.Error())
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
	if _, ok := req.URL.Query()["pretty"]; ok || s.agent.config.DevMode {
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
	return s.uiDir != "" || s.agent.config.EnableUI
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
		fmt.Fprint(resp, "Consul Agent")
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

// setTranslateAddr is used to set the address translation header. This is only
// present if the feature is active.
func setTranslateAddr(resp http.ResponseWriter, active bool) {
	if active {
		resp.Header().Set("X-Consul-Translate-Addresses", "true")
	}
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
			fmt.Fprint(resp, "Invalid wait time")
			return true
		}
		b.MaxQueryTime = dur
	}
	if idx := query.Get("index"); idx != "" {
		index, err := strconv.ParseUint(idx, 10, 64)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest) // 400
			fmt.Fprint(resp, "Invalid index")
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
		fmt.Fprint(resp, "Cannot specify ?stale with ?consistent, conflicting semantics.")
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

// parseMetaFilter is used to parse the ?node-meta=key:value query parameter, used for
// filtering results to nodes with the given metadata key/value
func (s *HTTPServer) parseMetaFilter(req *http.Request) map[string]string {
	if filterList, ok := req.URL.Query()["node-meta"]; ok {
		filters := make(map[string]string)
		for _, filter := range filterList {
			key, value := parseMetaPair(filter)
			filters[key] = value
		}
		return filters
	}
	return nil
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
