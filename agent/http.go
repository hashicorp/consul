package agent

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-cleanhttp"
	"github.com/mitchellh/mapstructure"
)

// MethodNotAllowedError should be returned by a handler when the HTTP method is not allowed.
type MethodNotAllowedError struct {
	Method string
	Allow  []string
}

func (e MethodNotAllowedError) Error() string {
	return fmt.Sprintf("method %s not allowed", e.Method)
}

// HTTPServer provides an HTTP api for an agent.
type HTTPServer struct {
	*http.Server
	ln        net.Listener
	agent     *Agent
	blacklist *Blacklist

	// proto is filled by the agent to "http" or "https".
	proto string
}

// endpoint is a Consul-specific HTTP handler that takes the usual arguments in
// but returns a response object and error, both of which are handled in a
// common manner by Consul's HTTP server.
type endpoint func(resp http.ResponseWriter, req *http.Request) (interface{}, error)

// unboundEndpoint is an endpoint method on a server.
type unboundEndpoint func(s *HTTPServer, resp http.ResponseWriter, req *http.Request) (interface{}, error)

// endpoints is a map from URL pattern to unbound endpoint.
var endpoints map[string]unboundEndpoint

// registerEndpoint registers a new endpoint, which should be done at package
// init() time.
func registerEndpoint(pattern string, fn unboundEndpoint) {
	if endpoints == nil {
		endpoints = make(map[string]unboundEndpoint)
	}
	if endpoints[pattern] != nil {
		panic(fmt.Errorf("Pattern %q is already registered", pattern))
	}
	endpoints[pattern] = fn
}

// wrappedMux hangs on to the underlying mux for unit tests.
type wrappedMux struct {
	mux     *http.ServeMux
	handler http.Handler
}

// ServeHTTP implements the http.Handler interface.
func (w *wrappedMux) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	w.handler.ServeHTTP(resp, req)
}

// handler is used to attach our handlers to the mux
func (s *HTTPServer) handler(enableDebug bool) http.Handler {
	mux := http.NewServeMux()

	// handleFuncMetrics takes the given pattern and handler and wraps to produce
	// metrics based on the pattern and request.
	handleFuncMetrics := func(pattern string, handler http.HandlerFunc) {
		// Get the parts of the pattern. We omit any initial empty for the
		// leading slash, and put an underscore as a "thing" placeholder if we
		// see a trailing slash, which means the part after is parsed. This lets
		// us distinguish from things like /v1/query and /v1/query/<query id>.
		var parts []string
		for i, part := range strings.Split(pattern, "/") {
			if part == "" {
				if i == 0 {
					continue
				}
				part = "_"
			}
			parts = append(parts, part)
		}

		// Register the wrapper, which will close over the expensive-to-compute
		// parts from above.
		// TODO (kyhavlov): Convert this to utilize metric labels in a major release
		wrapper := func(resp http.ResponseWriter, req *http.Request) {
			start := time.Now()
			handler(resp, req)
			key := append([]string{"http", req.Method}, parts...)
			metrics.MeasureSince(append([]string{"consul"}, key...), start)
			metrics.MeasureSince(key, start)
		}
		mux.HandleFunc(pattern, wrapper)
	}

	mux.HandleFunc("/", s.Index)
	for pattern, fn := range endpoints {
		thisFn := fn
		bound := func(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
			return thisFn(s, resp, req)
		}
		handleFuncMetrics(pattern, s.wrap(bound))
	}
	if enableDebug {
		handleFuncMetrics("/debug/pprof/", pprof.Index)
		handleFuncMetrics("/debug/pprof/cmdline", pprof.Cmdline)
		handleFuncMetrics("/debug/pprof/profile", pprof.Profile)
		handleFuncMetrics("/debug/pprof/symbol", pprof.Symbol)
	}

	// Use the custom UI dir if provided.
	if s.agent.config.UIDir != "" {
		mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir(s.agent.config.UIDir))))
	} else if s.agent.config.EnableUI {
		mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(assetFS())))
	}

	// Wrap the whole mux with a handler that bans URLs with non-printable
	// characters.
	return &wrappedMux{
		mux:     mux,
		handler: cleanhttp.PrintablePathCheckHandler(mux, nil),
	}
}

// nodeName returns the node name of the agent
func (s *HTTPServer) nodeName() string {
	return s.agent.config.NodeName
}

// aclEndpointRE is used to find old ACL endpoints that take tokens in the URL
// so that we can redact them. The ACL endpoints that take the token in the URL
// are all of the form /v1/acl/<verb>/<token>, and can optionally include query
// parameters which are indicated by a question mark. We capture the part before
// the token, the token, and any query parameters after, and then reassemble as
// $1<hidden>$3 (the token in $2 isn't used), which will give:
//
// /v1/acl/clone/foo           -> /v1/acl/clone/<hidden>
// /v1/acl/clone/foo?token=bar -> /v1/acl/clone/<hidden>?token=<hidden>
//
// The query parameter in the example above is obfuscated like any other, after
// this regular expression is applied, so the regular expression substitution
// results in:
//
// /v1/acl/clone/foo?token=bar -> /v1/acl/clone/<hidden>?token=bar
//                                ^---- $1 ----^^- $2 -^^-- $3 --^
//
// And then the loop that looks for parameters called "token" does the last
// step to get to the final redacted form.
var (
	aclEndpointRE = regexp.MustCompile("^(/v1/acl/[^/]+/)([^?]+)([?]?.*)$")
)

// wrap is used to wrap functions to make them more convenient
func (s *HTTPServer) wrap(handler endpoint) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		setHeaders(resp, s.agent.config.HTTPResponseHeaders)
		setTranslateAddr(resp, s.agent.config.TranslateWANAddrs)

		// Obfuscate any tokens from appearing in the logs
		formVals, err := url.ParseQuery(req.URL.RawQuery)
		if err != nil {
			s.agent.logger.Printf("[ERR] http: Failed to decode query: %s from=%s", err, req.RemoteAddr)
			resp.WriteHeader(http.StatusInternalServerError)
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
		logURL = aclEndpointRE.ReplaceAllString(logURL, "$1<hidden>$3")

		if s.blacklist.Block(req.URL.Path) {
			errMsg := "Endpoint is blocked by agent configuration"
			s.agent.logger.Printf("[ERR] http: Request %s %v, error: %v from=%s", req.Method, logURL, err, req.RemoteAddr)
			resp.WriteHeader(http.StatusForbidden)
			fmt.Fprint(resp, errMsg)
			return
		}

		isMethodNotAllowed := func(err error) bool {
			_, ok := err.(MethodNotAllowedError)
			return ok
		}

		handleErr := func(err error) {
			s.agent.logger.Printf("[ERR] http: Request %s %v, error: %v from=%s", req.Method, logURL, err, req.RemoteAddr)
			switch {
			case acl.IsErrPermissionDenied(err) || acl.IsErrNotFound(err):
				resp.WriteHeader(http.StatusForbidden)
				fmt.Fprint(resp, err.Error())
			case structs.IsErrRPCRateExceeded(err):
				resp.WriteHeader(http.StatusTooManyRequests)
			case isMethodNotAllowed(err):
				// RFC2616 states that for 405 Method Not Allowed the response
				// MUST include an Allow header containing the list of valid
				// methods for the requested resource.
				// https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html
				resp.Header()["Allow"] = err.(MethodNotAllowedError).Allow
				resp.WriteHeader(http.StatusMethodNotAllowed) // 405
				fmt.Fprint(resp, err.Error())
			default:
				resp.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(resp, err.Error())
			}
		}

		// Invoke the handler
		start := time.Now()
		defer func() {
			s.agent.logger.Printf("[DEBUG] http: Request %s %v (%v) from=%s", req.Method, logURL, time.Since(start), req.RemoteAddr)
		}()
		obj, err := handler(resp, req)
		if err != nil {
			handleErr(err)
			return
		}
		if obj == nil {
			return
		}

		buf, err := s.marshalJSON(req, obj)
		if err != nil {
			handleErr(err)
			return
		}
		resp.Header().Set("Content-Type", "application/json")
		resp.Write(buf)
	}
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
	return s.agent.config.UIDir != "" || s.agent.config.EnableUI
}

// Renders a simple index page
func (s *HTTPServer) Index(resp http.ResponseWriter, req *http.Request) {
	// Check if this is a non-index path
	if req.URL.Path != "/" {
		resp.WriteHeader(http.StatusNotFound)
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
	if last < 0 {
		last = 0
	}
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
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Invalid wait time")
			return true
		}
		b.MaxQueryTime = dur
	}
	if idx := query.Get("index"); idx != "" {
		index, err := strconv.ParseUint(idx, 10, 64)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
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
		resp.WriteHeader(http.StatusBadRequest)
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

	// Set the default ACLToken
	*token = s.agent.tokens.UserToken()
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
			key, value := ParseMetaPair(filter)
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
