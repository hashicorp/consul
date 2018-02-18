package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/logger"
)

var expectedEndpoints = []struct {
	methods, uri string
}{
	{"OPTIONS,PUT", "/v1/acl/bootstrap"},
	{"OPTIONS,PUT", "/v1/acl/create"},
	{"OPTIONS,PUT", "/v1/acl/update"},
	{"OPTIONS,PUT", "/v1/acl/destroy/"},
	{"OPTIONS,GET", "/v1/acl/info/"},
	{"OPTIONS,PUT", "/v1/acl/clone/"},
	{"OPTIONS,GET", "/v1/acl/list"},
	{"OPTIONS,GET", "/v1/acl/replication"},
	{"OPTIONS,PUT", "/v1/agent/token/"},
	{"OPTIONS,GET", "/v1/agent/self"},
	{"OPTIONS,GET", "/v1/agent/members"},
	{"OPTIONS,PUT", "/v1/agent/check/deregister/"},
	{"OPTIONS,PUT", "/v1/agent/check/fail/"},
	{"OPTIONS,PUT", "/v1/agent/check/pass/"},
	{"OPTIONS,PUT", "/v1/agent/check/register"},
	{"OPTIONS,PUT", "/v1/agent/check/update/"},
	{"OPTIONS,PUT", "/v1/agent/check/warn/"},
	{"OPTIONS,GET", "/v1/agent/checks"},
	{"OPTIONS,PUT", "/v1/agent/force-leave/"},
	{"OPTIONS,PUT", "/v1/agent/join/"},
	{"OPTIONS,PUT", "/v1/agent/leave"},
	{"OPTIONS,PUT", "/v1/agent/maintenance"},
	{"OPTIONS,GET", "/v1/agent/metrics"},
	// {"GET", "/v1/agent/monitor"}, // requires LogWriter. Hangs if LogWriter is provided
	{"OPTIONS,PUT", "/v1/agent/reload"},
	{"OPTIONS,PUT", "/v1/agent/service/deregister/"},
	{"OPTIONS,PUT", "/v1/agent/service/maintenance/"},
	{"OPTIONS,PUT", "/v1/agent/service/register"},
	{"OPTIONS,GET", "/v1/agent/services"},
	{"OPTIONS,GET", "/v1/catalog/datacenters"},
	{"OPTIONS,PUT", "/v1/catalog/deregister"},
	{"OPTIONS,GET", "/v1/catalog/node/"},
	{"OPTIONS,GET", "/v1/catalog/nodes"},
	{"OPTIONS,PUT", "/v1/catalog/register"},
	{"OPTIONS,GET", "/v1/catalog/service/"},
	{"OPTIONS,GET", "/v1/catalog/services"},
	{"OPTIONS,GET", "/v1/coordinate/datacenters"},
	{"OPTIONS,GET", "/v1/coordinate/nodes"},
	{"OPTIONS,GET", "/v1/coordinate/node/"},
	{"OPTIONS,PUT", "/v1/event/fire/"},
	{"OPTIONS,GET", "/v1/event/list"},
	{"OPTIONS,GET", "/v1/health/checks/"},
	{"OPTIONS,GET", "/v1/health/node/"},
	{"OPTIONS,GET", "/v1/health/service/"},
	{"OPTIONS,GET", "/v1/health/state/"},
	{"OPTIONS,GET", "/v1/internal/ui/node/"},
	{"OPTIONS,GET", "/v1/internal/ui/nodes"},
	{"OPTIONS,GET", "/v1/internal/ui/services"},
	{"OPTIONS,GET,PUT,DELETE", "/v1/kv/"},
	{"OPTIONS,GET,PUT", "/v1/operator/autopilot/configuration"},
	{"OPTIONS,GET", "/v1/operator/autopilot/health"},
	{"OPTIONS,GET,POST,PUT,DELETE", "/v1/operator/keyring"},
	{"OPTIONS,GET", "/v1/operator/raft/configuration"},
	{"OPTIONS,DELETE", "/v1/operator/raft/peer"},
	{"OPTIONS,GET,POST", "/v1/query"},
	{"OPTIONS,GET,PUT,DELETE", "/v1/query/"},
	{"OPTIONS,GET", "/v1/query/xxx/execute"},
	{"OPTIONS,GET", "/v1/query/xxx/explain"},
	{"OPTIONS,PUT", "/v1/session/create"},
	{"OPTIONS,PUT", "/v1/session/destroy/"},
	{"OPTIONS,GET", "/v1/session/info/"},
	{"OPTIONS,GET", "/v1/session/list"},
	{"OPTIONS,GET", "/v1/session/node/"},
	{"OPTIONS,PUT", "/v1/session/renew/"},
	{"OPTIONS,GET,PUT", "/v1/snapshot"},
	{"OPTIONS,GET", "/v1/status/leader"},
	// {"GET", "/v1/status/peers"},// hangs
	{"OPTIONS,PUT", "/v1/txn"},
}

func TestHTTPAPI_MethodNotAllowed_OSS(t *testing.T) {

	a := NewTestAgent(t.Name(), `acl_datacenter = "dc1"`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()

	all := []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"}
	client := http.Client{}

	for _, tt := range expectedEndpoints {
		for _, m := range all {
			t.Run(m+" "+tt.uri, func(t *testing.T) {
				uri := fmt.Sprintf("http://%s%s", a.HTTPAddr(), tt.uri)
				req, _ := http.NewRequest(m, uri, nil)
				resp, err := client.Do(req)
				if err != nil {
					t.Fatal("client.Do failed: ", err)
				}

				allowed := strings.Contains(tt.methods, m)
				if allowed && resp.StatusCode == http.StatusMethodNotAllowed {
					t.Fatalf("method allowed: got status code %d want any other code", resp.StatusCode)
				}
				if !allowed && resp.StatusCode != http.StatusMethodNotAllowed {
					t.Fatalf("method not allowed: got status code %d want %d", resp.StatusCode, http.StatusMethodNotAllowed)
				}
			})
		}
	}
}

func TestHTTPAPI_OptionMethod_OSS(t *testing.T) {
	a := NewTestAgent(t.Name(), `acl_datacenter = "dc1"`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()

	for _, tt := range expectedEndpoints {
		t.Run("OPTIONS "+tt.uri, func(t *testing.T) {
			uri := fmt.Sprintf("http://%s%s", a.HTTPAddr(), tt.uri)
			req, _ := http.NewRequest("OPTIONS", uri, nil)
			resp := httptest.NewRecorder()
			a.srv.Handler.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("options request: got status code %d want %d", resp.Code, http.StatusOK)
			}

			optionsStr := resp.Header().Get("Allow")
			if optionsStr == "" {
				t.Fatalf("options request: got empty 'Allow' header")
			} else if optionsStr != tt.methods {
				t.Fatalf("options request: got 'Allow' header value of %s want %s", optionsStr, tt.methods)
			}
		})
	}
}
