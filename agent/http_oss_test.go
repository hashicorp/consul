package agent

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/consul/logger"
)

func TestHTTPAPI_MethodNotAllowed_OSS(t *testing.T) {
	tests := []struct {
		methods, uri string
	}{
		{"PUT", "/v1/acl/bootstrap"},
		{"PUT", "/v1/acl/create"},
		{"PUT", "/v1/acl/update"},
		{"PUT", "/v1/acl/destroy/"},
		{"GET", "/v1/acl/info/"},
		{"PUT", "/v1/acl/clone/"},
		{"GET", "/v1/acl/list"},
		{"GET", "/v1/acl/replication"},
		{"PUT", "/v1/agent/token/"},
		{"GET", "/v1/agent/self"},
		{"GET", "/v1/agent/members"},
		{"PUT", "/v1/agent/check/deregister/"},
		{"PUT", "/v1/agent/check/fail/"},
		{"PUT", "/v1/agent/check/pass/"},
		{"PUT", "/v1/agent/check/register"},
		{"PUT", "/v1/agent/check/update/"},
		{"PUT", "/v1/agent/check/warn/"},
		{"GET", "/v1/agent/checks"},
		{"PUT", "/v1/agent/force-leave/"},
		{"PUT", "/v1/agent/join/"},
		{"PUT", "/v1/agent/leave"},
		{"PUT", "/v1/agent/maintenance"},
		{"GET", "/v1/agent/metrics"},
		// {"GET", "/v1/agent/monitor"}, // requires LogWriter. Hangs if LogWriter is provided
		{"PUT", "/v1/agent/reload"},
		{"PUT", "/v1/agent/service/deregister/"},
		{"PUT", "/v1/agent/service/maintenance/"},
		{"PUT", "/v1/agent/service/register"},
		{"GET", "/v1/agent/services"},
		{"GET", "/v1/catalog/datacenters"},
		{"PUT", "/v1/catalog/deregister"},
		{"GET", "/v1/catalog/node/"},
		{"GET", "/v1/catalog/nodes"},
		{"PUT", "/v1/catalog/register"},
		{"GET", "/v1/catalog/service/"},
		{"GET", "/v1/catalog/services"},
		{"GET", "/v1/coordinate/datacenters"},
		{"GET", "/v1/coordinate/nodes"},
		{"GET", "/v1/coordinate/node/"},
		{"PUT", "/v1/event/fire/"},
		{"GET", "/v1/event/list"},
		{"GET", "/v1/health/checks/"},
		{"GET", "/v1/health/node/"},
		{"GET", "/v1/health/service/"},
		{"GET", "/v1/health/state/"},
		{"GET", "/v1/internal/ui/node/"},
		{"GET", "/v1/internal/ui/nodes"},
		{"GET", "/v1/internal/ui/services"},
		{"GET PUT DELETE", "/v1/kv/"},
		{"GET PUT", "/v1/operator/autopilot/configuration"},
		{"GET", "/v1/operator/autopilot/health"},
		{"GET POST PUT DELETE", "/v1/operator/keyring"},
		{"GET", "/v1/operator/raft/configuration"},
		{"DELETE", "/v1/operator/raft/peer"},
		{"GET POST", "/v1/query"},
		{"GET PUT DELETE", "/v1/query/"},
		{"GET", "/v1/query/xxx/execute"},
		{"GET", "/v1/query/xxx/explain"},
		{"PUT", "/v1/session/create"},
		{"PUT", "/v1/session/destroy/"},
		{"GET", "/v1/session/info/"},
		{"GET", "/v1/session/list"},
		{"GET", "/v1/session/node/"},
		{"PUT", "/v1/session/renew/"},
		{"GET PUT", "/v1/snapshot"},
		{"GET", "/v1/status/leader"},
		// {"GET", "/v1/status/peers"},// hangs
		{"PUT", "/v1/txn"},
	}

	a := NewTestAgent(t.Name(), `acl_datacenter = "dc1"`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()

	all := []string{"GET", "PUT", "POST", "DELETE", "HEAD"}
	client := http.Client{}

	for _, tt := range tests {
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
