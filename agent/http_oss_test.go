package agent

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/consul/logger"
)

// extra endpoints that should be tested, and their allowed methods
var extraTestEndpoints = map[string][]string{
	"/v1/query":             []string{"GET", "POST"},
	"/v1/query/":            []string{"GET", "PUT", "DELETE"},
	"/v1/query/xxx/execute": []string{"GET"},
	"/v1/query/xxx/explain": []string{"GET"},
}

// These endpoints are ignored in unit testing for response codes
var ignoredEndpoints = []string{"/v1/status/peers", "/v1/agent/monitor", "/v1/agent/reload"}

// These have custom logic
var customEndpoints = []string{"/v1/query", "/v1/query/"}

// includePathInTest returns whether this path should be ignored for the purpose of testing its response code
func includePathInTest(path string) bool {
	ignored := false
	for _, p := range ignoredEndpoints {
		if p == path {
			ignored = true
			break
		}
	}
	for _, p := range customEndpoints {
		if p == path {
			ignored = true
			break
		}
	}

	return !ignored
}

func TestHTTPAPI_MethodNotAllowed_OSS(t *testing.T) {

	a := NewTestAgent(t.Name(), `acl_datacenter = "dc1"`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()

	all := []string{"GET", "PUT", "POST", "DELETE", "HEAD", "OPTIONS"}
	client := http.Client{}

	testMethodNotAllowed := func(method string, path string, allowedMethods []string) {
		t.Run(method+" "+path, func(t *testing.T) {
			uri := fmt.Sprintf("http://%s%s", a.HTTPAddr(), path)
			req, _ := http.NewRequest(method, uri, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal("client.Do failed: ", err)
			}
			defer resp.Body.Close()

			allowed := method == "OPTIONS"
			for _, allowedMethod := range allowedMethods {
				if allowedMethod == method {
					allowed = true
					break
				}
			}

			if allowed && resp.StatusCode == http.StatusMethodNotAllowed {
				t.Fatalf("method allowed: got status code %d want any other code", resp.StatusCode)
			}
			if !allowed && resp.StatusCode != http.StatusMethodNotAllowed {
				t.Fatalf("method not allowed: got status code %d want %d", resp.StatusCode, http.StatusMethodNotAllowed)
			}
		})
	}

	for path, methods := range extraTestEndpoints {
		for _, method := range all {
			testMethodNotAllowed(method, path, methods)
		}
	}

	for path, methods := range allowedMethods {
		if includePathInTest(path) {
			for _, method := range all {
				testMethodNotAllowed(method, path, methods)
			}
		}
	}
}

func TestHTTPAPI_OptionMethod_OSS(t *testing.T) {
	a := NewTestAgent(t.Name(), `acl_datacenter = "dc1"`)
	a.Agent.LogWriter = logger.NewLogWriter(512)
	defer a.Shutdown()

	testOptionMethod := func(path string, methods []string) {
		t.Run("OPTIONS "+path, func(t *testing.T) {
			uri := fmt.Sprintf("http://%s%s", a.HTTPAddr(), path)
			req, _ := http.NewRequest("OPTIONS", uri, nil)
			resp := httptest.NewRecorder()
			a.srv.Handler.ServeHTTP(resp, req)
			allMethods := append([]string{"OPTIONS"}, methods...)

			if resp.Code != http.StatusOK {
				t.Fatalf("options request: got status code %d want %d", resp.Code, http.StatusOK)
			}

			optionsStr := resp.Header().Get("Allow")
			if optionsStr == "" {
				t.Fatalf("options request: got empty 'Allow' header")
			} else if optionsStr != strings.Join(allMethods, ",") {
				t.Fatalf("options request: got 'Allow' header value of %s want %s", optionsStr, allMethods)
			}
		})
	}

	for path, methods := range extraTestEndpoints {
		testOptionMethod(path, methods)
	}
	for path, methods := range allowedMethods {
		if includePathInTest(path) {
			testOptionMethod(path, methods)
		}
	}
}
