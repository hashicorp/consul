package hcp

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	hcpgnm "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/client/global_network_manager_service"
	gnmmod "github.com/hashicorp/hcp-sdk-go/clients/cloud-global-network-manager-service/preview/2022-02-15/models"
	"github.com/hashicorp/hcp-sdk-go/resource"
)

type TestEndpoint struct {
	Methods    []string
	PathSuffix string
	Handler    func(r *http.Request, cluster resource.Resource) (interface{}, error)
}

type MockHCPServer struct {
	mu       sync.Mutex
	handlers map[string]TestEndpoint

	servers map[string]*gnmmod.HashicorpCloudGlobalNetworkManager20220215Server
}

var basePathRe = regexp.MustCompile("/global-network-manager/[^/]+/organizations/([^/]+)/projects/([^/]+)/clusters/([^/]+)/([^/]+.*)")

func NewMockHCPServer() *MockHCPServer {
	s := &MockHCPServer{
		handlers: make(map[string]TestEndpoint),
		servers:  make(map[string]*gnmmod.HashicorpCloudGlobalNetworkManager20220215Server),
	}
	// Define endpoints in this package
	s.AddEndpoint(TestEndpoint{
		Methods:    []string{"POST"},
		PathSuffix: "agent/server-state",
		Handler:    s.handleStatus,
	})
	s.AddEndpoint(TestEndpoint{
		Methods:    []string{"POST"},
		PathSuffix: "agent/discover",
		Handler:    s.handleDiscover,
	})
	return s
}

// AddEndpoint allows adding additional endpoints from other packages e.g.
// bootstrap (which can't be merged into one package due to dependency cycles).
// It's not safe to call this concurrently with any other call to AddEndpoint or
// ServeHTTP.
func (s *MockHCPServer) AddEndpoint(e TestEndpoint) {
	s.handlers[e.PathSuffix] = e
}

func (s *MockHCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.URL.Path == "/oauth2/token" {
		mockTokenResponse(w)
		return
	}

	matches := basePathRe.FindStringSubmatch(r.URL.Path)
	if matches == nil || len(matches) < 5 {
		w.WriteHeader(404)
		log.Printf("ERROR 404: %s %s\n", r.Method, r.URL.Path)
		return
	}

	cluster := resource.Resource{
		ID:           matches[3],
		Type:         "cluster",
		Organization: matches[1],
		Project:      matches[2],
	}
	found := false
	var resp interface{}
	var err error
	for _, e := range s.handlers {
		if e.PathSuffix == matches[4] {
			found = true
			if !enforceMethod(w, r, e.Methods) {
				return
			}
			resp, err = e.Handler(r, cluster)
			break
		}
	}
	if !found {
		w.WriteHeader(404)
		log.Printf("ERROR 404: %s %s\n", r.Method, r.URL.Path)
		return
	}
	if err != nil {
		errResponse(w, err)
		return
	}

	if resp == nil {
		// no response body
		log.Printf("OK 204: %s %s\n", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	bs, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		errResponse(w, err)
		return
	}

	log.Printf("OK 200: %s %s\n", r.Method, r.URL.Path)
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(bs)
}

func enforceMethod(w http.ResponseWriter, r *http.Request, methods []string) bool {
	for _, m := range methods {
		if strings.EqualFold(r.Method, m) {
			return true
		}
	}
	// No match, sent 4xx
	w.WriteHeader(http.StatusMethodNotAllowed)
	log.Printf("ERROR 405: bad method (not in %v): %s %s\n", methods, r.Method, r.URL.Path)
	return false
}

func mockTokenResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	w.Write([]byte(`{"access_token": "token", "token_type": "Bearer"}`))
}

func (s *MockHCPServer) handleStatus(r *http.Request, cluster resource.Resource) (interface{}, error) {
	var req hcpgnm.AgentPushServerStateBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, err
	}
	log.Printf("STATUS UPDATE: server=%s version=%s leader=%v hasLeader=%v healthy=%v tlsCertExpiryDays=%1.0f",
		req.ServerState.Name,
		req.ServerState.Version,
		req.ServerState.Raft.IsLeader,
		req.ServerState.Raft.KnownLeader,
		req.ServerState.Autopilot.Healthy,
		time.Until(time.Time(req.ServerState.TLS.CertExpiry)).Hours()/24,
	)
	s.servers[req.ServerState.Name] = &gnmmod.HashicorpCloudGlobalNetworkManager20220215Server{
		GossipPort: req.ServerState.GossipPort,
		ID:         req.ServerState.ID,
		LanAddress: req.ServerState.LanAddress,
		Name:       req.ServerState.Name,
		RPCPort:    req.ServerState.RPCPort,
	}
	return "{}", nil
}

func (s *MockHCPServer) handleDiscover(r *http.Request, cluster resource.Resource) (interface{}, error) {
	servers := make([]*gnmmod.HashicorpCloudGlobalNetworkManager20220215Server, len(s.servers))
	for _, server := range s.servers {
		servers = append(servers, server)
	}

	return gnmmod.HashicorpCloudGlobalNetworkManager20220215AgentDiscoverResponse{Servers: servers}, nil
}

func errResponse(w http.ResponseWriter, err error) {
	log.Printf("ERROR 500: %s\n", err)
	w.WriteHeader(500)
	w.Write([]byte(fmt.Sprintf(`{"error": %q}`, err.Error())))
}
